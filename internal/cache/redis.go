package cache

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/redis/go-redis/v9"
)

const (
	hashKeyEmployees                string = "employees"
	hashKeyEmployeesSearch          string = "employees_search"
	hashKeySleep                    string = "sleep"
	hashKeyInProgressEmployees      string = "in_progress_employees"
	hashKeyInProgressSleeps         string = "in_progress_sleeps"
	hashKeyInProgressEmployeesMutex string = "in_progress_employees_mutex"
	hashKeyInProgressSleepsMutex    string = "in_progress_sleeps_mutex"
	hashKeyNotFound                 string = "not_found_employees"
	hashKeyNotFoundMutex            string = "not_found_mutex"
)

type redisCache struct {
	sync.WaitGroup
	redisClient *redis.Client
	config      struct {
		address                 string
		port                    string
		password                string
		database                int
		timeout                 time.Duration
		mutexDisabled           bool
		mutexExpiration         time.Duration
		mutexRetryInterval      time.Duration
		inProgressPruneInterval time.Duration
		inProgressTTL           time.Duration
		inProgressEnabled       bool
		notFoundPruneInterval   time.Duration
		notFoundTTL             time.Duration
		notFoundEnabled         bool
		cacheTTL                time.Duration
	}
	ctx       context.Context
	ctxCancel context.CancelFunc
	utilities.Logger
}

func NewRedis(parameters ...any) interface {
	internal.Configurer
	internal.Opener
	internal.Clearer
	Cache
} {
	c := &redisCache{}
	for _, parameter := range parameters {
		switch p := parameter.(type) {
		case utilities.Logger:
			c.Logger = p
		}
	}
	return c
}

func (c *redisCache) launchPruneSetRead() {
	started := make(chan struct{})
	c.Add(1)
	go func() {
		defer c.Done()

		pruneEmployeesSetFx := func() {
			c.Lock(hashKeyInProgressEmployeesMutex)
			defer c.Unlock(hashKeyInProgressEmployeesMutex)

			var fieldsToDelete []string

			hscanIter := c.redisClient.HScan(c.ctx, hashKeyInProgressEmployees, 0, "*", 0).Iterator()
			for hscanIter.Next(c.ctx) {
				field := hscanIter.Val()
				t, _ := strconv.ParseInt(hscanIter.Val(), 10, 64)
				if time.Since(time.Unix(0, t)) > c.config.inProgressTTL {
					fieldsToDelete = append(fieldsToDelete, field)
				}
			}
			if err := hscanIter.Err(); err != nil {
				return
			}
			if len(fieldsToDelete) > 0 {
				_, _ = c.redisClient.HDel(c.ctx, hashKeyInProgressEmployees, fieldsToDelete...).Result()
			}
		}
		pruneSleepsSetFx := func() {
			c.Lock(hashKeyInProgressSleepsMutex)
			defer c.Unlock(hashKeyInProgressSleepsMutex)

			var fieldsToDelete []string

			hscanIter := c.redisClient.HScan(c.ctx, hashKeyInProgressSleeps, 0, "*", 0).Iterator()
			for hscanIter.Next(c.ctx) {
				field := hscanIter.Val()
				t, _ := strconv.ParseInt(hscanIter.Val(), 10, 64)
				if time.Since(time.Unix(0, t)) > c.config.inProgressTTL {
					fieldsToDelete = append(fieldsToDelete, field)
				}
			}
			if err := hscanIter.Err(); err != nil {
				return
			}
			if len(fieldsToDelete) > 0 {
				_, _ = c.redisClient.HDel(c.ctx, hashKeyInProgressSleeps, fieldsToDelete...).Result()
			}
		}
		tPrune := time.NewTicker(c.config.inProgressPruneInterval)
		defer tPrune.Stop()
		close(started)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-tPrune.C:
				pruneEmployeesSetFx()
				pruneSleepsSetFx()
			}
		}
	}()
	<-started
}

func (c *redisCache) launchPruneNotFound() {
	started := make(chan struct{})
	c.Add(1)
	go func() {
		defer c.Done()

		pruneFx := func() {
			c.Lock(hashKeyNotFoundMutex)
			defer c.Unlock(hashKeyNotFoundMutex)

			var fieldsToDelete []string

			hscanIter := c.redisClient.HScan(c.ctx, hashKeyNotFound, 0, "*", 0).Iterator()
			for hscanIter.Next(c.ctx) {
				field := hscanIter.Val()
				t, _ := strconv.ParseInt(hscanIter.Val(), 10, 64)
				if time.Since(time.Unix(0, t)) > c.config.notFoundTTL {
					fieldsToDelete = append(fieldsToDelete, field)
				}
			}
			if err := hscanIter.Err(); err != nil {
				return
			}
			if len(fieldsToDelete) > 0 {
				_, _ = c.redisClient.HDel(c.ctx, hashKeyNotFound, fieldsToDelete...).Result()
			}
		}
		tPrune := time.NewTicker(c.config.notFoundPruneInterval)
		defer tPrune.Stop()
		close(started)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-tPrune.C:
				pruneFx()
			}
		}
	}()
	<-started
}

func (r *redisCache) Lock(hashKey string) {
	if r.config.mutexDisabled {
		return
	}
	lockFx := func() bool {
		result, err := r.redisClient.SetNX(r.ctx, hashKey,
			true, r.config.mutexExpiration).Result()
		if err != nil {
			return false
		}
		return result
	}
	if lockFx() {
		return
	}
	tRetry := time.NewTicker(r.config.mutexRetryInterval)
	defer tRetry.Stop()
	for {
		select {
		case <-tRetry.C:
			if lockFx() {
				return
			}
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *redisCache) Unlock(hashKey string) {
	const script = `
			local key = KEYS[1]
			local expected_value = ARGV[1]

			local current_value = redis.call('GET', key)

			if current_value == expected_value then
			    return redis.call('DEL', key)
			else
		    	return 0 -- Key not deleted (value did not match)
			end
		`

	if r.config.mutexDisabled {
		return
	}
	item, err := r.redisClient.Eval(r.ctx, script,
		[]string{hashKey}, true).Result()
	if err != nil {
		return
	}
	i, ok := item.(int64)
	if !ok {
		return
	}
	if i != 1 {
		panic("attempted to unlock an unlocked mutex")
	}
}

func (c *redisCache) Configure(envs map[string]string) error {
	c.config.mutexExpiration = 10 * time.Second
	c.config.mutexRetryInterval = time.Second
	if s, ok := envs["CACHE_PRUNE_INTERVAL"]; ok {
		inProgressPruneInterval, _ := strconv.Atoi(s)
		c.config.inProgressPruneInterval = time.Second * time.Duration(inProgressPruneInterval)
	}
	if s, ok := envs["CACHE_SET_READ_TTL"]; ok {
		inProgressTTL, _ := strconv.Atoi(s)
		c.config.inProgressTTL = time.Second * time.Duration(inProgressTTL)
	}
	if inProgressEnabled, ok := envs["CACHE_ENABLE_IN_PROGRESS"]; ok {
		c.config.inProgressEnabled, _ = strconv.ParseBool(inProgressEnabled)
	}
	if redisAddress, ok := envs["REDIS_ADDRESS"]; ok {
		c.config.address = redisAddress
	}
	if redisPort, ok := envs["REDIS_PORT"]; ok {
		c.config.port = redisPort
	}
	if redisPassword, ok := envs["REDIS_PASSWORD"]; ok {
		c.config.password = redisPassword
	}
	if redisDatabase, ok := envs["REDIS_DATABASE"]; ok {
		i, _ := strconv.ParseInt(redisDatabase, 10, 64)
		c.config.database = int(i)
	}
	if redisTimeout, ok := envs["REDIS_TIMEOUT"]; ok {
		i, _ := strconv.ParseInt(redisTimeout, 10, 64)
		c.config.timeout = time.Duration(i) * time.Second
	}
	if s, ok := envs["CACHE_REDIS_MUTEX_EXPIRATION"]; ok {
		mutexExpiration, _ := strconv.Atoi(s)
		c.config.mutexExpiration = time.Second * time.Duration(mutexExpiration)
	}
	if s, ok := envs["REDIS_MUTEX_RETRY_INTERVAL"]; ok {
		mutexRetryInterval, _ := strconv.Atoi(s)
		c.config.mutexRetryInterval = time.Second * time.Duration(mutexRetryInterval)
	}
	if s, ok := envs["CACHE_PRUNE_INTERVAL"]; ok {
		inProgressPruneInterval, _ := strconv.Atoi(s)
		c.config.inProgressPruneInterval = time.Second * time.Duration(inProgressPruneInterval)
	}
	if s, ok := envs["CACHE_SET_READ_TTL"]; ok {
		inProgressTTL, _ := strconv.Atoi(s)
		c.config.inProgressTTL = time.Second * time.Duration(inProgressTTL)
	}
	if inProgressEnabled, ok := envs["CACHE_ENABLE_IN_PROGRESS"]; ok {
		c.config.inProgressEnabled, _ = strconv.ParseBool(inProgressEnabled)
	}
	if s, ok := envs["CACHE_NOT_FOUND_PRUNE_INTERVAL"]; ok {
		notFoundPruneInterval, _ := strconv.Atoi(s)
		c.config.notFoundPruneInterval = time.Second * time.Duration(notFoundPruneInterval)
	}
	if c.config.notFoundPruneInterval <= 0 {
		c.config.notFoundPruneInterval = 10 * time.Second
	}
	if s, ok := envs["CACHE_NOT_FOUND_TTL"]; ok {
		notFoundTTL, _ := strconv.Atoi(s)
		c.config.notFoundTTL = time.Second * time.Duration(notFoundTTL)
	}
	if notFoundEnabled, ok := envs["CACHE_NOT_FOUND_ENABLED"]; ok {
		c.config.notFoundEnabled, _ = strconv.ParseBool(notFoundEnabled)
	}
	if mutexDisabled, ok := envs["CACHE_REDIS_MUTEX_DISABLED"]; ok {
		c.config.mutexDisabled, _ = strconv.ParseBool(mutexDisabled)
	}
	c.config.cacheTTL = 5 * time.Second
	if s, ok := envs["CACHE_TTL"]; ok {
		i, _ := strconv.ParseInt(s, 10, 64)
		c.config.cacheTTL = time.Duration(i) * time.Second
	}
	return nil
}

func (c *redisCache) setExpiration() error {
	if _, err := c.redisClient.Expire(c.ctx, hashKeyEmployees,
		c.config.cacheTTL).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Expire(c.ctx, hashKeyEmployeesSearch,
		c.config.cacheTTL).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Expire(c.ctx, hashKeySleep,
		c.config.cacheTTL).Result(); err != nil {
		return err
	}
	return nil
}

func (c *redisCache) Open(ctx context.Context) error {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     net.JoinHostPort(c.config.address, c.config.port),
		Password: c.config.password,
		DB:       c.config.database,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return err
	}
	c.redisClient = redisClient
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
	if err := c.setExpiration(); err != nil {
		return err
	}
	if c.config.inProgressEnabled {
		c.launchPruneSetRead()
		c.Info(ctx, "cache: in progress enabled")
	}
	if c.config.notFoundEnabled {
		c.launchPruneNotFound()
		c.Info(ctx, "cache: not found enabled")
	}
	if c.config.mutexDisabled {
		c.Info(ctx, "cache: redis mutex disabled")
	}
	return nil
}

func (c *redisCache) Close(ctx context.Context) error {
	if c.config.inProgressEnabled {
		c.ctxCancel()
		c.Wait()
	}
	if err := c.redisClient.Close(); err != nil {
		c.Error(ctx, "error while shutting down redis client: %s", err)
	}
	return nil
}

func (c *redisCache) Clear(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	if _, err := c.redisClient.Del(ctx, hashKeyEmployees).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyEmployeesSearch).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyInProgressEmployees).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyInProgressEmployeesMutex).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyNotFound).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyNotFoundMutex).Result(); err != nil {
		return err
	}
	return nil
}

func (c *redisCache) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	key := fmt.Sprint(empNo)
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	value, err := c.redisClient.HGet(ctx, hashKeyEmployees, key).Result()
	if err != nil {
		switch {
		default:
			return nil, err
		case errors.Is(err, redis.Nil):
			if !c.config.inProgressEnabled {
				return nil, ErrEmployeeNotCached
			}
			c.Lock(hashKeyInProgressEmployeesMutex)
			defer c.Unlock(hashKeyInProgressEmployeesMutex)
			tNow := time.Now().UnixNano()
			result, err := c.redisClient.HSetNX(ctx, hashKeyInProgressEmployees, key,
				fmt.Sprint(tNow)).Result()
			if err != nil {
				return nil, fmt.Errorf("erorr while setting employee (%s) read in progress: %w", key, err)
			}
			if !result {
				return nil, ErrEmployeeReadAlreadySet
			}
			return nil, ErrEmployeeReadSet
		}
	}
	employee := &data.Employee{}
	if err := employee.UnmarshalBinary([]byte(value)); err != nil {
		return nil, err
	}
	return employee, nil
}

func (c *redisCache) EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	searchKey, err := search.ToKey()
	if err != nil {
		return nil, err
	}
	value, err := c.redisClient.HGet(ctx, hashKeyEmployeesSearch, searchKey).Result()
	if err != nil {
		return nil, err
	}
	if searchKey == "" || value == "" {
		if !c.config.inProgressEnabled {
			return nil, ErrEmployeeSearchNotCached
		}
		c.Lock(hashKeyInProgressEmployeesMutex)
		defer c.Unlock(hashKeyInProgressEmployeesMutex)
		tNow := time.Now().UnixNano()
		result, err := c.redisClient.HSetNX(ctx, hashKeyInProgressEmployees, searchKey,
			fmt.Sprint(tNow)).Result()
		if err != nil {
			return nil, fmt.Errorf("erorr while setting employee search in progress: %w", err)
		}
		if !result {
			return nil, ErrEmployeesSearchAlreadySet
		}
		return nil, ErrEmployeesSearchSet
	}
	empNos := strings.Split(value, ",")
	employees := make([]*data.Employee, 0, len(empNos))
	for _, empNo := range empNos {
		value, err := c.redisClient.HGet(ctx, hashKeyEmployees, fmt.Sprint(empNo)).Result()
		if err != nil {
			return nil, err
		}
		employee := &data.Employee{}
		if err := employee.UnmarshalBinary([]byte(value)); err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, nil
}

func (c *redisCache) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	searchKey, err := search.ToKey()
	if err != nil {
		return ErrSearchKey(err)
	}
	empNos := make([]string, 0, len(employees))
	for _, employee := range employees {
		bytes, err := employee.MarshalBinary()
		if err != nil {
			return err
		}
		if _, err := c.redisClient.HSet(ctx, hashKeyEmployees,
			fmt.Sprint(employee.EmpNo), string(bytes)).Result(); err != nil {
			return err
		}
		empNos = append(empNos, fmt.Sprint(employee.EmpNo))
	}
	if _, err := c.redisClient.HSet(ctx, hashKeyEmployeesSearch, searchKey,
		strings.Join(empNos, ",")).Result(); err != nil {
		return err
	}
	if c.config.inProgressEnabled {
		c.Lock(hashKeyInProgressEmployeesMutex)
		defer c.Unlock(hashKeyInProgressEmployeesMutex)

		fieldsToDelete := append(empNos, searchKey)
		_, _ = c.redisClient.HDel(ctx, hashKeyInProgressEmployees, fieldsToDelete...).Result()
	}
	return nil
}

func (c *redisCache) EmployeesDelete(ctx context.Context, e ...int64) error {
	var empNos []string

	if len(e) <= 0 {
		return nil
	}
	for _, empNo := range e {
		empNos = append(empNos, fmt.Sprint(empNo))
	}
	if _, err := c.redisClient.HDel(ctx, hashKeyEmployees,
		empNos...).Result(); err != nil {
		return err
	}
	if c.config.inProgressEnabled {
		c.Lock(hashKeyInProgressEmployeesMutex)
		defer c.Unlock(hashKeyInProgressEmployeesMutex)

		_, _ = c.redisClient.HDel(ctx, hashKeyEmployees,
			empNos...).Result()
	}
	return nil
}

func (c *redisCache) EmployeesNotFoundWrite(ctx context.Context, search data.EmployeeSearch, empNos ...int64) error {
	if !c.config.notFoundEnabled {
		return nil
	}
	searchKey, err := search.ToKey()
	if err != nil {
		return ErrSearchKey(err)
	}
	c.Lock(hashKeyNotFoundMutex)
	defer c.Unlock(hashKeyNotFoundMutex)
	tNow := time.Now().UnixNano()
	if _, err := c.redisClient.HSetNX(ctx, hashKeyNotFound, searchKey,
		fmt.Sprint(tNow)).Result(); err != nil {
		return fmt.Errorf("erorr while setting employee search not found: %w", err)
	}
	for _, empNo := range empNos {
		if _, err := c.redisClient.HSetNX(ctx, hashKeyNotFound, fmt.Sprint(empNo),
			fmt.Sprint(tNow)).Result(); err != nil {
			return fmt.Errorf("erorr while setting employee not found: %w", err)
		}
	}
	return nil
}

func (c *redisCache) SleepRead(ctx context.Context, sleepId string) (*data.Sleep, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	value, err := c.redisClient.HGet(ctx, hashKeySleep, sleepId).Result()
	if err != nil {
		switch {
		default:
			return nil, err
		case errors.Is(err, redis.Nil):
			if !c.config.inProgressEnabled {
				return nil, ErrSleepNotCached
			}
			c.Lock(hashKeyInProgressSleepsMutex)
			defer c.Unlock(hashKeyInProgressSleepsMutex)
			tNow := time.Now().UnixNano()
			result, err := c.redisClient.HSetNX(ctx, hashKeyInProgressSleeps, sleepId,
				fmt.Sprint(tNow)).Result()
			if err != nil {
				return nil, fmt.Errorf("erorr while setting sleep (%s) read in progress: %w", sleepId, err)
			}
			if !result {
				return nil, ErrSleepReadAlreadySet
			}
			return nil, ErrSleepReadSet
		}
	}
	sleep := &data.Sleep{}
	if err := sleep.UnmarshalBinary([]byte(value)); err != nil {
		return nil, err
	}
	return sleep, nil
}

func (c *redisCache) SleepWrite(ctx context.Context, sleep *data.Sleep) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	bytes, err := sleep.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err := c.redisClient.HSet(ctx, hashKeySleep,
		sleep.Id, string(bytes)).Result(); err != nil {
		return err
	}
	if c.config.inProgressEnabled {
		c.Lock(hashKeyInProgressSleepsMutex)
		defer c.Unlock(hashKeyInProgressSleepsMutex)
		_, _ = c.redisClient.HDel(ctx, hashKeyInProgressSleeps, sleep.Id).Result()
	}
	return nil
}

func (c *redisCache) SleepsDelete(ctx context.Context, sleepIds ...string) error {
	if len(sleepIds) <= 0 {
		return nil
	}
	if _, err := c.redisClient.HDel(ctx, hashKeyEmployees,
		sleepIds...).Result(); err != nil {
		return err
	}
	if c.config.inProgressEnabled {
		c.Lock(hashKeyInProgressEmployeesMutex)
		defer c.Unlock(hashKeyInProgressEmployeesMutex)
		_, _ = c.redisClient.HDel(ctx, hashKeyEmployees,
			sleepIds...).Result()
	}
	return nil
}

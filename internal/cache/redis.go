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
	hashKeyEmployees       string = "employees"
	hashKeySearch          string = "search"
	hashKeyInProgress      string = "in_progress_employees"
	hashKeyInProgressMutex string = "in_progress_mutex"
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
		inProgressPruneInterval time.Duration
		inProgressTTL           time.Duration
		inProgressEnabled       bool
		mutexExpiration         time.Duration
		mutexRetryInterval      time.Duration
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

		pruneFx := func() {
			c.Lock()
			defer c.Unlock()

			var fieldsToDelete []string

			hscanIter := c.redisClient.HScan(c.ctx, hashKeyInProgress, 0, "*", 0).Iterator()
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
				_, _ = c.redisClient.HDel(c.ctx, hashKeyInProgress, fieldsToDelete...).Result()
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
				pruneFx()
			}
		}
	}()
	<-started
}

func (r *redisCache) Lock() {
	lockFx := func() bool {
		result, err := r.redisClient.SetNX(r.ctx, hashKeyInProgressMutex,
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

func (r *redisCache) Unlock() {
	script := `
			local key = KEYS[1]
			local expected_value = ARGV[1]

			local current_value = redis.call('GET', key)

			if current_value == expected_value then
			    return redis.call('DEL', key)
			else
		    	return 0 -- Key not deleted (value did not match)
			end
		`
	item, err := r.redisClient.Eval(r.ctx, script,
		[]string{hashKeyInProgressMutex}, true).Result()
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
	if s, ok := envs["CACHE_REDIS_MUTEX_EXPIRATION"]; ok {
		mutexExpiration, _ := strconv.Atoi(s)
		c.config.mutexExpiration = time.Second * time.Duration(mutexExpiration)
	}
	if s, ok := envs["REDIS_MUTEX_RETRY_INTERVAL"]; ok {
		mutexRetryInterval, _ := strconv.Atoi(s)
		c.config.mutexRetryInterval = time.Second * time.Duration(mutexRetryInterval)
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
	if c.config.inProgressEnabled {
		c.ctx, c.ctxCancel = context.WithCancel(context.Background())
		c.launchPruneSetRead()
	}
	return nil
}

func (c *redisCache) Close(ctx context.Context) error {
	if c.config.inProgressEnabled {
		c.ctxCancel()
		c.Wait()
	}
	if err := c.redisClient.Close(); err != nil {
		fmt.Printf("error while shutting down redis client: %s\n", err)
	}
	return nil
}

func (c *redisCache) Clear(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	if _, err := c.redisClient.Del(ctx, hashKeyEmployees).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeySearch).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyInProgress).Result(); err != nil {
		return err
	}
	if _, err := c.redisClient.Del(ctx, hashKeyInProgressMutex).Result(); err != nil {
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
			c.Lock()
			defer c.Unlock()
			tNow := time.Now().UnixNano()
			result, err := c.redisClient.HSetNX(ctx, hashKeyInProgress, key,
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
	value, err := c.redisClient.HGet(ctx, hashKeySearch, searchKey).Result()
	if err != nil {
		return nil, err
	}
	if searchKey == "" || value == "" {
		if !c.config.inProgressEnabled {
			return nil, ErrEmployeeSearchNotCached
		}
		c.Lock()
		defer c.Unlock()
		tNow := time.Now().UnixNano()
		result, err := c.redisClient.HSetNX(ctx, hashKeyInProgress, searchKey,
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
		return fmt.Errorf("error while creating search key: %w\n", err)
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
	if _, err := c.redisClient.HSet(ctx, hashKeySearch, searchKey,
		strings.Join(empNos, ",")).Result(); err != nil {
		return err
	}
	if c.config.inProgressEnabled {
		c.Lock()
		defer c.Unlock()

		fieldsToDelete := append(empNos, searchKey)
		_, _ = c.redisClient.HDel(ctx, hashKeyInProgress, fieldsToDelete...).Result()
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
		c.Lock()
		defer c.Unlock()

		_, _ = c.redisClient.HDel(ctx, hashKeyEmployees,
			empNos...).Result()
	}
	return nil
}

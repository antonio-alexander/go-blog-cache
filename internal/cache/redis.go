package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/redis/go-redis/v9"
)

const (
	hashKeyEmployees string = "employees" //strings
	hashKeySearch    string = "search"
)

type redisCache struct {
	sync.RWMutex
	sync.WaitGroup
	logger       utilities.Logger
	cacheCounter utilities.CacheCounter
	ctx          context.Context
	cancel       context.CancelFunc
	redisClient  *redis.Client
	config       struct {
		Address  string        `json:"address"`
		Port     string        `json:"port"`
		Password string        `json:"password"`
		Database int           `json:"database"`
		Timeout  time.Duration `json:"timeout"`
	}
	initialized bool
	configured  bool
}

func NewRedis(parameters ...interface{}) Cache {
	c := &redisCache{}
	for _, p := range parameters {
		switch p := p.(type) {
		case utilities.CacheCounter:
			c.cacheCounter = p
		case utilities.Logger:
			c.logger = p
		}
	}
	return c
}

func (c *redisCache) toRedisOptions() *redis.Options {
	address := c.config.Address
	if c.config.Port != "" {
		address = address + ":" + c.config.Port
	}
	return &redis.Options{
		Addr:     address,
		Password: c.config.Password,
		DB:       c.config.Database,
	}
}

func (c *redisCache) Error(correlationId, format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Error(correlationId, format, v...)
	}
}

func (c *redisCache) Trace(correlationId, format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Trace(correlationId, format, v...)
	}
}

func (c *redisCache) IncrementHit(item any) (hitCount int) {
	if c.cacheCounter != nil {
		switch v := item.(type) {
		case string:
			return c.cacheCounter.IncrementHit(fmt.Sprintf("employee_search_%s", v))
		case int64:
			return c.cacheCounter.IncrementHit(fmt.Sprintf("employee_%d", v))
		}
	}
	return -1
}

func (c *redisCache) IncrementMiss(item any) (hitCount int) {
	if c.cacheCounter != nil {
		switch v := item.(type) {
		case string:
			return c.cacheCounter.IncrementMiss(fmt.Sprintf("employee_search_%s", v))
		case int64:
			return c.cacheCounter.IncrementMiss(fmt.Sprintf("employee_%d", v))
		}
	}
	return -1
}

func (c *redisCache) Configure(envs map[string]string) error {
	if redisAddress, ok := envs["REDIS_ADDRESS"]; ok {
		c.config.Address = redisAddress
	}
	if redisPort, ok := envs["REDIS_PORT"]; ok {
		c.config.Port = redisPort
	}
	if redisPassword, ok := envs["REDIS_PASSWORD"]; ok {
		c.config.Password = redisPassword
	}
	if redisDatabase, ok := envs["REDIS_DATABASE"]; ok {
		i, _ := strconv.ParseInt(redisDatabase, 10, 64)
		c.config.Database = int(i)
	}
	if redisTimeout, ok := envs["REDIS_TIMEOUT"]; ok {
		i, _ := strconv.ParseInt(redisTimeout, 10, 64)
		c.config.Timeout = time.Duration(i) * time.Second
	}
	c.configured = true
	return nil
}

func (c *redisCache) Open(correlationId string) error {
	c.Lock()
	defer c.Unlock()

	redisClient := redis.NewClient(c.toRedisOptions())
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return err
	}
	c.redisClient = redisClient
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.initialized = true
	return nil
}

func (c *redisCache) Close(correlationId string) error {
	c.Lock()
	defer c.Unlock()

	c.cancel()
	c.Wait()
	if err := c.redisClient.Close(); err != nil {
		c.Error(correlationId, "error while shutting down redis client: %s\n", err)
	}
	c.initialized = false
	return nil
}

func (c *redisCache) Clear(correlationId string, ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	//clear employees
	empNos, err := c.redisClient.HKeys(ctx, hashKeyEmployees).Result()
	if err != nil {
		return err
	}
	for _, empNos := range empNos {
		if _, err := c.redisClient.HDel(ctx, hashKeyEmployees,
			empNos).Result(); err != nil {
			return err
		}
	}
	//clear searches
	searches, err := c.redisClient.HKeys(ctx, hashKeySearch).Result()
	if err != nil {
		return err
	}
	for _, search := range searches {
		if _, err := c.redisClient.HDel(ctx, hashKeySearch,
			search).Result(); err != nil {
			return err
		}
	}
	return nil
}

func (c *redisCache) EmployeeRead(correlationId string, ctx context.Context, empNo int64) (*data.Employee, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	value, err := c.redisClient.HGet(ctx, hashKeyEmployees, fmt.Sprint(empNo)).Result()
	if err != nil {
		c.Trace(correlationId, "cache miss for employee: %d", empNo)
		c.IncrementMiss(empNo)
		return nil, err
	}
	employee := &data.Employee{}
	if err := employee.UnmarshalBinary([]byte(value)); err != nil {
		c.Trace(correlationId, "cache miss for employee: %d", empNo)
		c.IncrementMiss(empNo)
		return nil, err
	}
	c.IncrementHit(empNo)
	c.Trace(correlationId, "cache hit for employee: %d", empNo)
	return employee, nil
}

func (c *redisCache) EmployeesRead(correlationId string, ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	//generate search key
	searchKey, err := searchToKey(search)
	if err != nil {
		return nil, err
	}
	// check if search key exists
	value, err := c.redisClient.HGet(ctx, hashKeySearch, searchKey).Result()
	if err != nil {
		c.IncrementMiss(searchKey)
		return nil, err
	}
	if searchKey == "" || value == "" {
		c.IncrementMiss(search)
		c.Trace(correlationId, "cache miss for employee search: %s", searchKey)
		return nil, errors.New("search not cached")
	}
	//get employees
	empNos := strings.Split(value, ",")
	employees := make([]*data.Employee, 0, len(empNos))
	for _, empNo := range empNos {
		value, err := c.redisClient.HGet(ctx, hashKeyEmployees, fmt.Sprint(empNo)).Result()
		if err != nil {
			c.IncrementMiss(search)
			return nil, err
		}
		employee := &data.Employee{}
		if err := employee.UnmarshalBinary([]byte(value)); err != nil {
			c.IncrementMiss(search)
			return nil, err
		}
		employees = append(employees, employee)
	}
	c.Trace(correlationId, "cache hit for employee search: %s", searchKey)
	c.IncrementHit(searchKey)
	return employees, nil
}

func (c *redisCache) EmployeesWrite(correlationId string, ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	//cache search
	searchKey, err := searchToKey(search)
	if err != nil {
		c.Error(correlationId, "error while creating search key: %s", err)
		return err
	}
	empNos := make([]string, 0, len(employees))
	//cache employees
	for _, employee := range employees {
		bytes, err := employee.MarshalBinary()
		if err != nil {
			return err
		}
		if _, err := c.redisClient.HSet(ctx, hashKeyEmployees,
			fmt.Sprint(employee.EmpNo), string(bytes)).Result(); err != nil {
			return err
		}
		c.Trace(correlationId, "cached employee: %d", employee.EmpNo)
		empNos = append(empNos, fmt.Sprint(employee.EmpNo))
	}
	//cache search employees
	if _, err := c.redisClient.HSet(ctx, hashKeySearch, searchKey,
		strings.Join(empNos, ",")).Result(); err != nil {
		return err
	}
	c.Trace(correlationId, "cached employees search: %s", searchKey)
	return nil
}

func (c *redisCache) EmployeesDelete(correlationId string, ctx context.Context, empNos ...int64) error {
	for _, empNo := range empNos {
		if _, err := c.redisClient.HDel(ctx, hashKeyEmployees,
			fmt.Sprint(empNo)).Result(); err != nil {
			c.Error(correlationId, "error while invalidating cached employee: %s", err)
			return err
		}
		c.Trace(correlationId, "evicted cached employee: %d", empNo)
	}
	return nil
}

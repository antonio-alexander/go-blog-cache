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

	"github.com/redis/go-redis/v9"
)

const (
	hashKeyEmployees string = "employees" //strings
	hashKeySearch    string = "search"
)

type redisCache struct {
	sync.RWMutex
	sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	redisClient *redis.Client
	config      struct {
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
	return &redisCache{}
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

func (c *redisCache) Open() error {
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

func (c *redisCache) Close() error {
	c.Lock()
	defer c.Unlock()

	c.cancel()
	c.Wait()
	if err := c.redisClient.Close(); err != nil {
		fmt.Printf("error while shutting down redis client: %s\n", err)
	}
	c.initialized = false
	return nil
}

func (c *redisCache) Clear(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	//clear employees
	empNos, err := c.redisClient.HKeys(ctx, hashKeyEmployees).Result()
	if err != nil {
		return err
	}
	if _, err := c.redisClient.HDel(ctx, hashKeyEmployees,
		empNos...).Result(); err != nil {
		return err
	}
	//clear searches
	searches, err := c.redisClient.HKeys(ctx, hashKeySearch).Result()
	if err != nil {
		return err
	}
	if _, err := c.redisClient.HDel(ctx, hashKeySearch,
		searches...).Result(); err != nil {
		return err
	}
	return nil
}

func (c *redisCache) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	value, err := c.redisClient.HGet(ctx, hashKeyEmployees, fmt.Sprint(empNo)).Result()
	if err != nil {
		return nil, err
	}
	employee := &data.Employee{}
	if err := employee.UnmarshalBinary([]byte(value)); err != nil {
		return nil, err
	}
	return employee, nil
}

func (c *redisCache) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	//cache search
	searchKey, err := searchToKey(search)
	if err != nil {
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
		empNos = append(empNos, fmt.Sprint(employee.EmpNo))
	}
	//cache search employees
	if _, err := c.redisClient.HSet(ctx, hashKeySearch, searchKey,
		strings.Join(empNos, ",")).Result(); err != nil {
		return err
	}
	return nil
}

func (c *redisCache) EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
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
		return nil, err
	}
	if searchKey == "" || value == "" {
		return nil, errors.New("search not cached")
	}
	//get employees
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

func (c *redisCache) EmployeesDelete(ctx context.Context, empNos ...int64) error {
	for _, empNo := range empNos {
		if _, err := c.redisClient.HDel(ctx, hashKeyEmployees,
			fmt.Sprint(empNo)).Result(); err != nil {
			return err
		}
	}
	return nil
}

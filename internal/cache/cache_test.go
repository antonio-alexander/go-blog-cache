package cache_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/antonio-alexander/go-blog-big-data/internal"
	"github.com/antonio-alexander/go-blog-big-data/internal/cache"
	"github.com/antonio-alexander/go-blog-big-data/internal/data"
	"github.com/antonio-alexander/go-blog-big-data/internal/utilities"

	"github.com/antonio-alexander/go-stash/memory"
	"github.com/antonio-alexander/go-stash/redis"

	"github.com/stretchr/testify/assert"
)

var envs = map[string]string{
	//redis
	"REDIS_ADDRESS":  "localhost",
	"REDIS_PORT":     "6379",
	"REDIS_TIMEOUT":  "10",
	"REDIS_PASSWORD": "",
	"REDIS_DATABASE": "0",
	"REDIS_HASH_KEY": "go_blog_cache",
	//logging
	"LOGGING_LEVEL": "TRACE",
	//stash_memory
	"STASH_EVICTION_POLICY": "least_recently_used",
	"STASH_TIME_TO_LIVE":    "0",
	"STASH_MAX_SIZE":        "5242880", //5MB
	"STASH_DEBUG_ENABLED":   "false",
	"STASH_DEBUG_PREFIX":    "stash ",
	//stash_redis
	"STASH_EVICTION_RATE": "1", //1s
}

func init() {
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
}

type cacheTest struct {
	cache.Cache
	utilities.Logger
	utilities.CacheCounter
}

func newCacheTest(cacheType string) *cacheTest {
	var employeeCache cache.Cache

	logger := utilities.NewLogger()
	cacheCounter := utilities.NewCacheCounter()
	switch cacheType {
	case "memory":
		employeeCache = cache.NewMemory(logger, cacheCounter)
	case "redis":
		employeeCache = cache.NewRedis(logger, cacheCounter)
	case "stash-memory":
		stash := memory.New()
		employeeCache = cache.NewStash(logger, cacheCounter, stash)
	case "stash-redis":
		stash := redis.New()
		employeeCache = cache.NewStash(logger, cacheCounter, stash)
	}
	return &cacheTest{
		Cache:        employeeCache,
		Logger:       logger,
		CacheCounter: cacheCounter,
	}
}

func (c *cacheTest) Configure(envs map[string]string) error {
	if err := c.Cache.Configure(envs); err != nil {
		return err
	}
	if err := c.Logger.Configure(envs); err != nil {
		return err
	}
	return nil
}

func (c *cacheTest) Clear(correlationId string, ctx context.Context) error {
	if err := c.Cache.Clear(correlationId, ctx); err != nil {
		return err
	}
	return nil
}

func (c *cacheTest) Open(correlationId string) error {
	if err := c.Cache.Open(correlationId); err != nil {
		return err
	}
	return nil
}

func (c *cacheTest) Close(correlationId string) error {
	if err := c.Cache.Close(correlationId); err != nil {
		return err
	}
	return nil
}

func (c *cacheTest) testCache(t *testing.T) {
	var search data.EmployeeSearch

	// generate correlation id
	correlationId := internal.GenerateId()
	t.Logf("correlation id: %s", correlationId)

	//create employee
	employees := []*data.Employee{
		{
			EmpNo:     1,
			FirstName: internal.GenerateId(),
			LastName:  internal.GenerateId(),
		},
		{
			EmpNo:     2,
			FirstName: internal.GenerateId(),
			LastName:  internal.GenerateId(),
		},
		{
			EmpNo:     3,
			FirstName: internal.GenerateId(),
			LastName:  internal.GenerateId(),
		},
		{
			EmpNo:     4,
			FirstName: internal.GenerateId(),
			LastName:  internal.GenerateId(),
		},
		{
			EmpNo:     5,
			FirstName: internal.GenerateId(),
			LastName:  internal.GenerateId(),
		},
	}

	//create context
	ctx := context.TODO()

	//clear cache
	err := c.Clear(correlationId, ctx)
	assert.Nil(t, err)

	// write employees
	err = c.EmployeesWrite(correlationId, ctx, search, employees...)
	assert.Nil(t, err)

	// read employee[0]
	employeeRead, err := c.EmployeeRead(correlationId, ctx, employees[0].EmpNo)
	assert.Nil(t, err)
	assert.Equal(t, employees[0], employeeRead)

	// read employee[1]
	employeeRead, err = c.EmployeeRead(correlationId, ctx, employees[1].EmpNo)
	assert.Nil(t, err)
	assert.Equal(t, employees[1], employeeRead)

	// read employees
	for _, employee := range employees {
		search.EmpNos = append(search.EmpNos, employee.EmpNo)
	}
	_, err = c.EmployeesRead(correlationId, ctx, search)
	assert.NotNil(t, err)

	// write search
	err = c.EmployeesWrite(correlationId, ctx, search, employees...)
	assert.Nil(t, err)

	// read employees
	employeesRead, err := c.EmployeesRead(correlationId, ctx, search)
	assert.Nil(t, err)
	assert.Equal(t, len(employees), len(employeesRead))
	for _, employee := range employees {
		assert.Contains(t, employeesRead, employee)
	}

	// delete employee [1]
	err = c.EmployeesDelete(correlationId, ctx, employees[1].EmpNo)
	assert.Nil(t, err)

	//  attempt to read employee [1]
	employeeRead, err = c.EmployeeRead(correlationId, ctx, employees[1].EmpNo)
	assert.NotNil(t, err)
	assert.Nil(t, employeeRead)
}

func testCache(t *testing.T, cacheType string) {
	const correlationId string = "test_cache"

	c := newCacheTest(cacheType)

	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure test")
	}
	err = c.Open(correlationId)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open test")
	}
	defer func(correlationId string) {
		if err := c.Close(correlationId); err != nil {
			t.Logf("error while closing test: %s", err)
		}
	}(correlationId)
	t.Run("Cache", c.testCache)
}

func TestCacheMemory(t *testing.T) {
	testCache(t, "memory")
}

func TestCacheRedis(t *testing.T) {
	testCache(t, "redis")
}

func TestCacheStashMemory(t *testing.T) {
	testCache(t, "stash-memory")
}

func TestCacheStashRedis(t *testing.T) {
	testCache(t, "stash-redis")
}

package client_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/client"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/antonio-alexander/go-stash/memory"
	"github.com/antonio-alexander/go-stash/redis"

	"github.com/stretchr/testify/assert"
)

var (
	envs = map[string]string{
		//cache
		"REDIS_ADDRESS": "localhost",
		"REDIS_PORT":    "6379",
		"REDIS_TIMEOUT": "10",
		//client
		"CLIENT_ADDRESS":  "localhost",
		"CLIENT_PORT":     "8080",
		"CLIENT_PROTOCOL": "http",
		"CLIENT_TIMEOUT":  "10",
		"SSL_CA_FILE":     "",
		"SSL_KEY_FILE":    "",
		"SSL_CRT_FILE":    "",
		"CACHE_DISABLED":  "false",
		//logger
		"LOGGING_LEVEL": "trace",
	}
)

func init() {
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
}

type clientTest struct {
	cache        cache.Cache
	logger       utilities.Logger
	cacheCounter utilities.CacheCounter
	timers       utilities.Timers
	client.Client
}

func newClientTest(cacheType string) *clientTest {
	var employeeCache cache.Cache

	cacheCounter := utilities.NewCacheCounter()
	timers := utilities.NewTimers()
	logger := utilities.NewLogger()
	sql := sql.NewSql(logger)
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
	client := client.NewClient(sql, employeeCache, logger, timers)
	return &clientTest{
		cache:        employeeCache,
		logger:       logger,
		cacheCounter: cacheCounter,
		timers:       timers,
		Client:       client,
	}
}

func (c *clientTest) testClient(cacheDisabled bool) func(t *testing.T) {
	return func(t *testing.T) {

		//generate correlationId
		correlationId := internal.GenerateId()
		t.Logf("correlation id: %s", correlationId)

		// generate context
		ctx := context.TODO()

		// create employee
		birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
		firstName := internal.GenerateId()[:14]
		lastName := internal.GenerateId()[:16]
		gender := "M"
		employeeCreated, err := c.EmployeeCreate(correlationId, ctx, data.EmployeePartial{
			BirthDate: &birthDate,
			FirstName: &firstName,
			LastName:  &lastName,
			HireDate:  &hireDate,
			Gender:    &gender,
		})
		assert.Nil(t, err)
		assert.NotNil(t, employeeCreated)
		empNo := employeeCreated.EmpNo
		defer func(empNo int64) {
			_ = c.EmployeeDelete(correlationId, ctx, empNo)
		}(empNo)

		if !cacheDisabled {
			// validate that employee not in cache
			employeeCached, err := c.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err := c.EmployeeRead(correlationId, ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeCreated, employeeRead)

		// validate that employee in cache
		if !cacheDisabled {
			employeeCached, err := c.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeCreated, employeeCached)
		}

		// update employee
		updatedFirstName := internal.GenerateId()[:14]
		updatedLastName := internal.GenerateId()[:16]
		employeeUpdated, err := c.EmployeeUpdate(correlationId, ctx, empNo, data.EmployeePartial{
			FirstName: &updatedFirstName,
			LastName:  &updatedLastName,
		})
		assert.Nil(t, err)
		assert.NotNil(t, employeeUpdated)

		// validate that employee not in cache
		if !cacheDisabled {
			employeeCached, err := c.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err = c.EmployeeRead(correlationId, ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeUpdated, employeeRead)

		// validate that employee in cache
		if !cacheDisabled {
			employeeCached, err := c.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeUpdated, employeeCached)
		}

		// delete employee
		err = c.EmployeeDelete(correlationId, ctx, empNo)
		assert.Nil(t, err)
		if !cacheDisabled {
			// validate that employee not in cache
			employeeCached, err := c.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}
	}
}

func (c *clientTest) Configure(envs map[string]string) error {
	if err := c.logger.Configure(envs); err != nil {
		return err
	}
	if err := c.cache.Configure(envs); err != nil {
		return err
	}
	if err := c.Client.Configure(envs); err != nil {
		return err
	}
	return nil
}

func (c *clientTest) Open() error {
	correlationId := "client_test"
	if err := c.cache.Open(correlationId); err != nil {
		return err
	}
	if err := c.Client.Open(correlationId); err != nil {
		return err
	}
	return nil
}

func (c *clientTest) Close() error {
	correlationId := "client_test"
	if err := c.cache.Close(correlationId); err != nil {
		return err
	}
	if err := c.Client.Close(correlationId); err != nil {
		return err
	}
	return nil
}

func testClient(t *testing.T, cacheType string) {
	c := newClientTest(cacheType)

	cacheDisabled, _ := strconv.ParseBool(envs["CACHE_DISABLED"])
	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure testClient")
	}
	err = c.Open()
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open testClient")
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Logf("error while closing testClient: %s", err)
		}
	}()
	t.Run("Client", c.testClient(cacheDisabled))
}

func TestClientMemory(t *testing.T) {
	testClient(t, "memory")
}

func TestClientRedis(t *testing.T) {
	testClient(t, "redis")
}

func TestClientStashMemory(t *testing.T) {
	testClient(t, "stash-memory")
}

func TestClientStashRedis(t *testing.T) {
	testClient(t, "stash-redis")
}

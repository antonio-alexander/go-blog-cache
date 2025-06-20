package logic_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-big-data/internal"
	"github.com/antonio-alexander/go-blog-big-data/internal/cache"
	"github.com/antonio-alexander/go-blog-big-data/internal/data"
	"github.com/antonio-alexander/go-blog-big-data/internal/logic"
	"github.com/antonio-alexander/go-blog-big-data/internal/sql"
	"github.com/antonio-alexander/go-blog-big-data/internal/utilities"
	"github.com/antonio-alexander/go-stash/memory"
	"github.com/antonio-alexander/go-stash/redis"

	"github.com/stretchr/testify/assert"
)

var envs = map[string]string{
	//sql
	"DATABASE_HOST":          "localhost",
	"DATABASE_PORT":          "3306",
	"DATABASE_NAME":          "employees",
	"DATABASE_USER":          "mysql",
	"DATABASE_PASSWORD":      "mysql",
	"DATABASE_QUERY_TIMEOUT": "10",
	"DATABASE_PARSE_TIME":    "true",
	//redis
	"REDIS_ADDRESS":  "localhost",
	"REDIS_PORT":     "6379",
	"REDIS_TIMEOUT":  "10",
	"REDIS_PASSWORD": "",
	"REDIS_DATABASE": "0",
	"REDIS_HASH_KEY": "go_blog_cache",
	//logic
	"LOGIC_CACHE_ENABLED": "true",
	//logger
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

type logicTest struct {
	sql    *sql.Sql
	cache  cache.Cache
	logger utilities.Logger
	logic.Logic
}

func newLogicTest(cacheType string) *logicTest {
	var employeeCache cache.Cache

	sql := sql.NewSql()
	logger := utilities.NewLogger()
	switch cacheType {
	case "memory":
		employeeCache = cache.NewMemory(logger)
	case "redis":
		employeeCache = cache.NewRedis(logger)
	case "stash-memory":
		stash := memory.New()
		employeeCache = cache.NewStash(logger, stash)
	case "stash-redis":
		stash := redis.New()
		employeeCache = cache.NewStash(logger, stash)
	}
	logic := logic.NewLogic(sql, employeeCache, logger)
	return &logicTest{
		sql:    sql,
		cache:  employeeCache,
		logger: logger,
		Logic:  logic,
	}
}

func (l *logicTest) Configure(envs map[string]string) error {
	if err := l.sql.Configure(envs); err != nil {
		return err
	}
	if err := l.cache.Configure(envs); err != nil {
		return err
	}
	if err := l.logger.Configure(envs); err != nil {
		return err
	}
	if err := l.Logic.Configure(envs); err != nil {
		return err
	}
	return nil
}

func (l *logicTest) Open(correlationId string) error {
	if err := l.sql.Open(correlationId); err != nil {
		return err
	}
	if err := l.cache.Open(correlationId); err != nil {
		return err
	}
	if err := l.Logic.Open(correlationId); err != nil {
		return err
	}
	return nil
}

func (l *logicTest) Close(correlationId string) error {
	if err := l.sql.Close(correlationId); err != nil {
		return err
	}
	if err := l.cache.Close(correlationId); err != nil {
		return err
	}
	if err := l.Logic.Close(correlationId); err != nil {
		return err
	}
	return nil
}

func (l *logicTest) testLogic(cacheEnabled bool) func(t *testing.T) {
	return func(t *testing.T) {
		//generate correlationId string
		correlationId := internal.GenerateId()

		// generate context
		ctx := context.TODO()

		// create employee
		birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
		firstName := internal.GenerateId()[:14]
		lastName := internal.GenerateId()[:16]
		gender := "M"
		employeeCreated, err := l.EmployeeCreate(correlationId, ctx,
			data.EmployeePartial{
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
			_ = l.EmployeeDelete(correlationId, ctx, empNo)
		}(empNo)

		if cacheEnabled {
			// validate that employee not in cache
			employeeCached, err := l.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err := l.EmployeeRead(correlationId, ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeCreated, employeeRead)

		// validate that employee in cache
		if cacheEnabled {
			employeeCached, err := l.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeCreated, employeeCached)
		}

		// update employee
		updatedFirstName := internal.GenerateId()[:14]
		updatedLastName := internal.GenerateId()[:16]
		employeeUpdated, err := l.EmployeeUpdate(correlationId, ctx, empNo,
			data.EmployeePartial{
				FirstName: &updatedFirstName,
				LastName:  &updatedLastName,
			})
		assert.Nil(t, err)
		assert.NotNil(t, employeeUpdated)

		// validate that employee not in cache
		if cacheEnabled {
			employeeCached, err := l.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err = l.EmployeeRead(correlationId, ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeUpdated, employeeRead)

		// validate that employee in cache
		if cacheEnabled {
			employeeCached, err := l.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeUpdated, employeeCached)
		}

		// delete employee
		err = l.EmployeeDelete(correlationId, ctx, empNo)
		assert.Nil(t, err)

		if cacheEnabled {
			// validate that employee not in cache
			employeeCached, err := l.cache.EmployeeRead(correlationId, ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}
	}
}

func testLogic(t *testing.T, cacheType string) {
	const correlationId string = "test_logic"
	c := newLogicTest(cacheType)

	cacheEnabled, _ := strconv.ParseBool(envs["LOGIC_CACHE_ENABLED"])
	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure testLogic")
	}
	err = c.Open(correlationId)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open testLogic")
	}
	defer func() {
		if err := c.Close(correlationId); err != nil {
			t.Logf("error while closing testLogic: %s", err)
		}
	}()
	t.Run("Logic", c.testLogic(cacheEnabled))
}

func TestLogicMemory(t *testing.T) {
	testLogic(t, "memory")
}

func TestLogicRedis(t *testing.T) {
	testLogic(t, "redis")
}

func TestLogicStashMemory(t *testing.T) {
	testLogic(t, "stash-memory")
}

func TestLogicStashRedis(t *testing.T) {
	testLogic(t, "stash-redis")
}

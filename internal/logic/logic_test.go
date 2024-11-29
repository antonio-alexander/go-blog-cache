package logic_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"

	"github.com/stretchr/testify/assert"
)

var (
	envs = map[string]string{
		//sql
		"DATABASE_HOST":          "localhost",
		"DATABASE_PORT":          "3306",
		"DATABASE_NAME":          "employees",
		"DATABASE_USER":          "mysql",
		"DATABASE_PASSWORD":      "mysql",
		"DATABASE_QUERY_TIMEOUT": "10",
		"DATABASE_PARSE_TIME":    "true",
		//cache
		"REDIS_ADDRESS": "localhost",
		"REDIS_PORT":    "6379",
		"REDIS_TIMEOUT": "10",
		//logic
		"LOGIC_CACHE_ENABLED": "true",
	}
)

func init() {
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
}

type logicTest struct {
	sql   *sql.Sql
	cache cache.Cache
	*logic.Logic
}

func newLogicTest(cacheType string) *logicTest {
	var c cache.Cache

	sql := sql.NewSql()
	switch cacheType {
	case "memory":
		c = cache.NewMemory()
	case "redis":
		c = cache.NewRedis()
	}
	logic := logic.NewLogic(sql, c)
	return &logicTest{
		sql:   sql,
		cache: c,
		Logic: logic,
	}
}

func (l *logicTest) Configure(envs map[string]string) error {
	if err := l.sql.Configure(envs); err != nil {
		return err
	}
	if err := l.cache.Configure(envs); err != nil {
		return err
	}
	if err := l.Logic.Configure(envs); err != nil {
		return err
	}
	return nil
}

func (l *logicTest) Open() error {
	if err := l.sql.Open(); err != nil {
		return err
	}
	if err := l.cache.Open(); err != nil {
		return err
	}
	if err := l.Logic.Open(); err != nil {
		return err
	}
	return nil
}

func (l *logicTest) Close() error {
	if err := l.sql.Close(); err != nil {
		return err
	}
	if err := l.cache.Close(); err != nil {
		return err
	}
	if err := l.Logic.Close(); err != nil {
		return err
	}
	return nil
}

func (l *logicTest) TestLogic(cacheEnabled bool) func(t *testing.T) {
	return func(t *testing.T) {
		// generate context
		ctx := context.TODO()

		// create employee
		birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
		firstName := internal.GenerateId()[:14]
		lastName := internal.GenerateId()[:16]
		gender := "M"
		employeeCreated, err := l.EmployeeCreate(ctx, data.EmployeePartial{
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
			_ = l.EmployeeDelete(ctx, empNo)
		}(empNo)

		if cacheEnabled {
			// validate that employee not in cache
			employeeCached, err := l.cache.EmployeeRead(ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err := l.EmployeeRead(ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeCreated, employeeRead)

		// validate that employee in cache
		if cacheEnabled {
			employeeCached, err := l.cache.EmployeeRead(ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeCreated, employeeCached)
		}

		// update employee
		updatedFirstName := internal.GenerateId()[:14]
		updatedLastName := internal.GenerateId()[:16]
		employeeUpdated, err := l.EmployeeUpdate(ctx, empNo, data.EmployeePartial{
			FirstName: &updatedFirstName,
			LastName:  &updatedLastName,
		})
		assert.Nil(t, err)
		assert.NotNil(t, employeeUpdated)

		// validate that employee not in cache
		if cacheEnabled {
			employeeCached, err := l.cache.EmployeeRead(ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err = l.EmployeeRead(ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeUpdated, employeeRead)

		// validate that employee in cache
		if cacheEnabled {
			employeeCached, err := l.cache.EmployeeRead(ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeUpdated, employeeCached)
		}

		// delete employee
		err = l.EmployeeDelete(ctx, empNo)
		assert.Nil(t, err)

		if cacheEnabled {
			// validate that employee not in cache
			employeeCached, err := l.cache.EmployeeRead(ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}
	}
}

func testLogic(t *testing.T, cacheType string) {
	c := newLogicTest(cacheType)

	cacheEnabled, _ := strconv.ParseBool(envs["LOGIC_CACHE_ENABLED"])
	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure testLogic")
	}
	err = c.Open()
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open testLogic")
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Logf("error while closing testLogic: %s", err)
		}
	}()
	t.Run("Logic", c.TestLogic(cacheEnabled))
}

func TestLogicMemory(t *testing.T) {
	testLogic(t, "memory")
}

func TestLogicRedis(t *testing.T) {
	testLogic(t, "redis")
}

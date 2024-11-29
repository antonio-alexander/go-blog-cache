package cache_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var envs = map[string]string{
	"REDIS_ADDRESS": "localhost",
	"REDIS_PORT":    "6379",
	"REDIS_TIMEOUT": "10",
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
}

func newCacheTest(cacheType string) *cacheTest {
	var employeeCache interface {
		cache.Cache
	}

	switch cacheType {
	case "memory":
		employeeCache = cache.NewMemory()
	case "redis":
		employeeCache = cache.NewRedis()
	}
	employeeCache.Configure(envs)
	return &cacheTest{
		Cache: employeeCache,
	}
}

func generateId() string {
	return uuid.Must(uuid.NewRandom()).String()
}

func (c *cacheTest) TestCache(t *testing.T) {
	var search data.EmployeeSearch

	//create employee
	employees := []*data.Employee{
		{
			EmpNo:     1,
			FirstName: generateId(),
			LastName:  generateId(),
		},
		{
			EmpNo:     2,
			FirstName: generateId(),
			LastName:  generateId(),
		},
		{
			EmpNo:     3,
			FirstName: generateId(),
			LastName:  generateId(),
		},
		{
			EmpNo:     4,
			FirstName: generateId(),
			LastName:  generateId(),
		},
		{
			EmpNo:     5,
			FirstName: generateId(),
			LastName:  generateId(),
		},
	}

	//create context
	ctx := context.TODO()

	//clear cache
	err := c.Clear(ctx)
	assert.Nil(t, err)

	//TODO: write employees
	err = c.EmployeesWrite(ctx, search, employees...)
	assert.Nil(t, err)

	//TODO: read employee[0]
	employeeRead, err := c.EmployeeRead(ctx, employees[0].EmpNo)
	assert.Nil(t, err)
	assert.Equal(t, employees[0], employeeRead)

	//TODO: read employee[1]
	employeeRead, err = c.EmployeeRead(ctx, employees[1].EmpNo)
	assert.Nil(t, err)
	assert.Equal(t, employees[1], employeeRead)

	//TODO: read employees
	for _, employee := range employees {
		search.EmpNos = append(search.EmpNos, employee.EmpNo)
	}
	_, err = c.EmployeesRead(ctx, search)
	assert.NotNil(t, err)

	//TODO: write search
	err = c.EmployeesWrite(ctx, search, employees...)
	assert.Nil(t, err)

	//TODO: read employees
	employeesRead, err := c.EmployeesRead(ctx, search)
	assert.Nil(t, err)
	assert.Equal(t, len(employees), len(employeesRead))
	for _, employee := range employees {
		assert.Contains(t, employeesRead, employee)
	}
}

func testCache(t *testing.T, cacheType string) {
	c := newCacheTest(cacheType)

	err := c.Open()
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open cache")
	}
	defer func() {
		c.Close()
	}()
	t.Run("Cache", c.TestCache)
}

func TestCacheMemory(t *testing.T) {
	testCache(t, "memory")
}

func TestCacheRedis(t *testing.T) {
	testCache(t, "redis")
}

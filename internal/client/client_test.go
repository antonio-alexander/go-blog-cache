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
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

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
	cache interface {
		internal.Opener
		internal.Configurer
		internal.Clearer
		cache.Cache
	}
	client interface {
		internal.Opener
		internal.Configurer
	}
	client.Client
}

func newClientTest(cacheType string) *clientTest {
	var c interface {
		internal.Opener
		internal.Configurer
		internal.Clearer
		cache.Cache
	}

	logger := utilities.NewLogger
	switch cacheType {
	case "memory":
		c = cache.NewMemory()
	case "redis":
		c = cache.NewRedis()
	}
	client := client.NewClient(c, logger)
	return &clientTest{
		cache:  c,
		Client: client,
	}
}

func (c *clientTest) Configure(envs map[string]string) error {
	if err := c.cache.Configure(envs); err != nil {
		return err
	}
	if err := c.client.Configure(envs); err != nil {
		return err
	}
	return nil
}

func (c *clientTest) Open(ctx context.Context) error {
	if err := c.cache.Open(ctx); err != nil {
		return err
	}
	if err := c.client.Open(ctx); err != nil {
		return err
	}
	return nil
}

func (c *clientTest) Close(ctx context.Context) error {
	if err := c.cache.Close(ctx); err != nil {
		return err
	}
	if err := c.client.Close(ctx); err != nil {
		return err
	}
	return nil
}

func (c *clientTest) TestClient(cacheDisabled bool) func(t *testing.T) {
	return func(t *testing.T) {
		// generate context
		ctx := context.TODO()

		// create employee
		birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
		firstName := internal.GenerateId()[:14]
		lastName := internal.GenerateId()[:16]
		gender := "M"
		employeeCreated, err := c.EmployeeCreate(ctx, data.EmployeePartial{
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
			_ = c.EmployeeDelete(ctx, empNo)
		}(empNo)

		if !cacheDisabled {
			// validate that employee not in cache
			employeeCached, err := c.cache.EmployeeRead(ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err := c.EmployeeRead(ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeCreated, employeeRead)

		// validate that employee in cache
		if !cacheDisabled {
			employeeCached, err := c.cache.EmployeeRead(ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeCreated, employeeCached)
		}

		// update employee
		updatedFirstName := internal.GenerateId()[:14]
		updatedLastName := internal.GenerateId()[:16]
		employeeUpdated, err := c.EmployeeUpdate(ctx, empNo, data.EmployeePartial{
			FirstName: &updatedFirstName,
			LastName:  &updatedLastName,
		})
		assert.Nil(t, err)
		assert.NotNil(t, employeeUpdated)

		// validate that employee not in cache
		if !cacheDisabled {
			employeeCached, err := c.cache.EmployeeRead(ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}

		// read employee
		employeeRead, err = c.EmployeeRead(ctx, empNo)
		assert.Nil(t, err)
		assert.NotNil(t, employeeRead)
		assert.Equal(t, employeeUpdated, employeeRead)

		// validate that employee in cache
		if !cacheDisabled {
			employeeCached, err := c.cache.EmployeeRead(ctx, empNo)
			assert.Nil(t, err)
			assert.NotNil(t, employeeCached)
			assert.Equal(t, employeeUpdated, employeeCached)
		}

		// delete employee
		err = c.EmployeeDelete(ctx, empNo)
		assert.Nil(t, err)
		if !cacheDisabled {
			// validate that employee not in cache
			employeeCached, err := c.cache.EmployeeRead(ctx, empNo)
			assert.NotNil(t, err)
			assert.Nil(t, employeeCached)
		}
	}
}

func testClient(t *testing.T, cacheType string) {
	c := newClientTest(cacheType)

	ctx := context.TODO()
	cacheDisabled, _ := strconv.ParseBool(envs["CACHE_DISABLED"])
	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure testClient")
	}
	err = c.Open(ctx)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open testClient")
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			t.Logf("error while closing testClient: %s", err)
		}
	}()
	t.Run("Client", c.TestClient(cacheDisabled))
}

func TestClientMemory(t *testing.T) {
	testClient(t, "memory")
}

func TestClientRedis(t *testing.T) {
	testClient(t, "redis")
}

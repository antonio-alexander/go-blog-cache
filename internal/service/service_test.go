package service_test

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"
	"github.com/antonio-alexander/go-blog-cache/internal/service"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"
	"github.com/antonio-alexander/go-stash/memory"
	"github.com/antonio-alexander/go-stash/redis"

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
		//service
		"SERVICE_ADDRESS":                "localhost",
		"SERVICE_PORT":                   "8000", //KIM: we don't want this to conflict
		"SERVICE_SHUTDOWN_TIMEOUT":       "30",
		"SERVICE_CORS_ALLOW_CREDENTIALS": "",
		"SERVICE_CORS_ALLOWED_ORIGINS":   "",
		"SERVICE_CORS_ALLOWED_METHODS":   "",
		"SERVICE_CORS_DISABLED":          "",
		"SERVICE_CORS_DEBUG":             "",
	}
)

func init() {
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
}

type serviceTest struct {
	sql          *sql.Sql
	cache        cache.Cache
	logger       utilities.Logger
	logic        logic.Logic
	timers       utilities.Timers
	cacheCounter utilities.CacheCounter
	client       *http.Client
	service.Service
	address string
}

func newServiceTest(cacheType string) *serviceTest {
	var employeeCache cache.Cache

	logger := utilities.NewLogger()
	cacheCounter := utilities.NewCacheCounter()
	timers := utilities.NewTimers()
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
	logic := logic.NewLogic(sql, employeeCache, logger)
	service := service.NewService(logic, logger, timers)
	return &serviceTest{
		sql:          sql,
		cache:        employeeCache,
		logic:        logic,
		logger:       logger,
		timers:       timers,
		cacheCounter: cacheCounter,
		client:       &http.Client{},
		Service:      service,
	}
}

func (s *serviceTest) testService(t *testing.T) {
	var request data.Request
	var response data.Response

	// create employee
	birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
	firstName := internal.GenerateId()[:14]
	lastName := internal.GenerateId()[:16]
	gender := "M"
	employeeCreated := &data.Employee{}
	request.EmployeePartial = data.EmployeePartial{
		BirthDate: &birthDate,
		FirstName: &firstName,
		LastName:  &lastName,
		HireDate:  &hireDate,
		Gender:    &gender,
	}
	response.Employee = employeeCreated
	uriEmployeeCreate := s.address + data.RouteEmployees
	_, err := internal.DoRequest(s.client, uriEmployeeCreate, http.MethodPut, &request, &response)
	assert.Nil(t, err)
	assert.NotZero(t, employeeCreated.EmpNo)
	empNo := employeeCreated.EmpNo
	defer func(empNo int64) {
		uri := fmt.Sprintf(s.address+data.RouteEmployeesEmpNof, empNo)
		_, _ = internal.DoRequest(s.client, uri, http.MethodDelete, nil)
	}(empNo)

	// read employee
	employeeRead := &data.Employee{}
	response.Employee = employeeRead
	uriEmployeeRead := fmt.Sprintf(s.address+data.RouteEmployeesEmpNof, empNo)
	_, err = internal.DoRequest(s.client, uriEmployeeRead, http.MethodGet, nil, &response)
	assert.Nil(t, err)
	assert.Equal(t, employeeCreated, employeeRead)

	// read employees
	search := data.EmployeeSearch{EmpNos: []int64{empNo}}
	uriEmployeesSearch := s.address + data.RouteEmployeesSearch
	_, err = internal.DoRequest(s.client, uriEmployeesSearch, http.MethodGet,
		search.ToParams(), &response)
	assert.Nil(t, err)
	assert.Equal(t, employeeCreated, employeeRead)

	// update employee
	updatedFirstName := internal.GenerateId()[:14]
	updatedLastName := internal.GenerateId()[:16]
	employeeUpdated := &data.Employee{}
	request.EmployeePartial = data.EmployeePartial{
		FirstName: &updatedFirstName,
		LastName:  &updatedLastName,
	}
	response.Employee = employeeUpdated
	uriEmployeeUpdate := fmt.Sprintf(s.address+data.RouteEmployeesEmpNof, empNo)
	_, err = internal.DoRequest(s.client, uriEmployeeUpdate, http.MethodPost, &request, &response)
	assert.Nil(t, err)

	// delete employee
	uriEmployeeDelete := fmt.Sprintf(s.address+data.RouteEmployeesEmpNof, empNo)
	_, err = internal.DoRequest(s.client, uriEmployeeDelete, http.MethodDelete, nil)
	assert.Nil(t, err)
}

func (s *serviceTest) Configure(envs map[string]string) error {
	if err := s.sql.Configure(envs); err != nil {
		return err
	}
	if err := s.cache.Configure(envs); err != nil {
		return err
	}
	if err := s.logic.Configure(envs); err != nil {
		return err
	}
	if err := s.logger.Configure(envs); err != nil {
		return err
	}
	if err := s.Service.Configure(envs); err != nil {
		return err
	}
	s.address = "http://" + envs["SERVICE_ADDRESS"]
	if port := envs["SERVICE_PORT"]; port != "" {
		s.address += ":" + port
	}
	return nil
}

func (s *serviceTest) Open(correlationId string) error {
	if err := s.sql.Open(correlationId); err != nil {
		return err
	}
	if err := s.cache.Open(correlationId); err != nil {
		return err
	}
	if err := s.logic.Open(correlationId); err != nil {
		return err
	}
	if err := s.Service.Open(correlationId); err != nil {
		return err
	}
	return nil
}

func (s *serviceTest) Close(correlationId string) error {
	if err := s.sql.Close(correlationId); err != nil {
		return err
	}
	if err := s.cache.Close(correlationId); err != nil {
		return err
	}
	if err := s.logic.Close(correlationId); err != nil {
		return err
	}
	if err := s.Service.Close(correlationId); err != nil {
		return err
	}
	return nil
}

func testService(t *testing.T, cacheType string) {
	const correlationId string = "test_service"
	c := newServiceTest(cacheType)

	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure testService")
	}
	err = c.Open(correlationId)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open testService")
	}
	defer func() {
		if err := c.Close(correlationId); err != nil {
			t.Logf("error while closing testService: %s", err)
		}
	}()
	t.Run("Service", c.testService)
}

func TestServiceMemory(t *testing.T) {
	testService(t, "memory")
}

func TestServiceRedis(t *testing.T) {
	testService(t, "redis")
}

func TestServiceStashMemory(t *testing.T) {
	testService(t, "stash-memory")
}

func TestServiceStashRedis(t *testing.T) {
	testService(t, "stash-redis")
}

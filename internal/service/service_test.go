package service_test

import (
	"context"
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

		//service
		"SERVICE_ADDRESS":                "localhost",
		"SERVICE_PORT":                   "8080",
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
	sql interface {
		internal.Configurer
		internal.Opener
		sql.Sql
	}
	cache interface {
		internal.Configurer
		internal.Opener
		cache.Cache
	}
	logic interface {
		internal.Configurer
		internal.Opener
		logic.Logic
	}
	service interface {
		internal.Configurer
		internal.Opener
	}
	client  *http.Client
	address string
}

func newServiceTest(cacheType string) *serviceTest {
	var c interface {
		internal.Opener
		internal.Configurer
		internal.Clearer
		cache.Cache
	}

	sql := sql.NewMySql()
	switch cacheType {
	case "memory":
		c = cache.NewMemory()
	case "redis":
		c = cache.NewRedis()
	}
	logic := logic.NewLogic(sql, c)
	service := service.NewService(logic)
	return &serviceTest{
		sql:     sql,
		cache:   c,
		logic:   logic,
		client:  &http.Client{},
		service: service,
	}
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
	if err := s.service.Configure(envs); err != nil {
		return err
	}
	s.address = "http://" + envs["SERVICE_ADDRESS"]
	if port := envs["SERVICE_PORT"]; port != "" {
		s.address += ":" + port
	}
	return nil
}

func (s *serviceTest) Open(ctx context.Context) error {
	if err := s.sql.Open(ctx); err != nil {
		return err
	}
	if err := s.cache.Open(ctx); err != nil {
		return err
	}
	if err := s.logic.Open(ctx); err != nil {
		return err
	}
	if err := s.service.Open(ctx); err != nil {
		return err
	}
	return nil
}

func (s *serviceTest) Close(ctx context.Context) error {
	if err := s.sql.Close(ctx); err != nil {
		return err
	}
	if err := s.cache.Close(ctx); err != nil {
		return err
	}
	if err := s.logic.Close(ctx); err != nil {
		return err
	}
	if err := s.service.Close(ctx); err != nil {
		return err
	}
	return nil
}

func (s *serviceTest) TestService(t *testing.T) {
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

func testService(t *testing.T, cacheType string) {
	c := newServiceTest(cacheType)

	ctx := context.TODO()
	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure testService")
	}
	err = c.Open(ctx)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open testService")
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			t.Logf("error while closing testService: %s", err)
		}
	}()
	t.Run("Service", c.TestService)
}

func TestServiceMemory(t *testing.T) {
	testService(t, "memory")
}

func TestServiceRedis(t *testing.T) {
	testService(t, "redis")
}

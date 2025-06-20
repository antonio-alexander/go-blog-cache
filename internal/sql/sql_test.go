package sql_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-big-data/internal"
	"github.com/antonio-alexander/go-blog-big-data/internal/data"
	"github.com/antonio-alexander/go-blog-big-data/internal/sql"
	"github.com/antonio-alexander/go-blog-big-data/internal/utilities"

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
		//logging
		"LOGGING_LEVEL": "TRACE",
	}
)

func init() {
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
}

type sqlTest struct {
	utilities.Logger
	*sql.Sql
}

func newSqlTest() *sqlTest {
	sql := sql.NewSql()
	logger := utilities.NewLogger()
	return &sqlTest{
		Sql:    sql,
		Logger: logger,
	}
}

func (s *sqlTest) testSql(t *testing.T) {
	// generate correlationId
	correlationId := internal.GenerateId()
	t.Logf("correlation id: %s", correlationId)

	// generate context
	ctx := context.TODO()

	// create employee
	birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
	firstName := internal.GenerateId()[:14]
	lastName := internal.GenerateId()[:16]
	gender := "M"
	employeeCreated, err := s.EmployeeCreate(correlationId, ctx, data.EmployeePartial{
		BirthDate: &birthDate,
		FirstName: &firstName,
		LastName:  &lastName,
		HireDate:  &hireDate,
		Gender:    &gender,
	})
	assert.Nil(t, err)
	assert.NotNil(t, employeeCreated)
	// assert.Equal(t, birthDate, employeeCreated.BirthDate)
	// assert.Equal(t, hireDate, employeeCreated.HireDate)
	assert.Equal(t, firstName, employeeCreated.FirstName)
	assert.Equal(t, lastName, employeeCreated.LastName)
	assert.Equal(t, gender, employeeCreated.Gender)
	empNo := employeeCreated.EmpNo
	defer func(empNo int64) {
		_ = s.EmployeeDelete(correlationId, ctx, empNo)
	}(empNo)

	// read employee
	employeeRead, err := s.EmployeeRead(correlationId, ctx, empNo)
	assert.Nil(t, err)
	assert.NotNil(t, employeeRead)
	assert.Equal(t, employeeCreated, employeeRead)

	// search employee
	employeesRead, err := s.EmployeesSearch(correlationId, ctx,
		data.EmployeeSearch{EmpNos: []int64{empNo}})
	assert.Nil(t, err)
	assert.NotEmpty(t, employeesRead)
	assert.Len(t, employeesRead, 1)
	assert.Contains(t, employeesRead, employeeCreated)

	// update employee
	updatedFirstName := internal.GenerateId()[:14]
	updatedLastName := internal.GenerateId()[:16]
	employeeUpdated, err := s.EmployeeUpdate(correlationId, ctx, empNo,
		data.EmployeePartial{
			FirstName: &updatedFirstName,
			LastName:  &updatedLastName,
		})
	assert.Nil(t, err)
	assert.NotNil(t, employeeUpdated)
	assert.NotEqual(t, firstName, employeeUpdated.FirstName)
	assert.NotEqual(t, lastName, employeeUpdated.LastName)
	assert.Equal(t, updatedFirstName, employeeUpdated.FirstName)
	assert.Equal(t, updatedLastName, employeeUpdated.LastName)
	// assert.Equal(t, birthDate, employeeUpdated.BirthDate)
	// assert.Equal(t, hireDate, employeeUpdated.HireDate)
	assert.Equal(t, gender, employeeUpdated.Gender)

	//  read employee again
	employeeRead, err = s.EmployeeRead(correlationId, ctx, empNo)
	assert.Nil(t, err)
	assert.NotNil(t, employeeRead)
	assert.Equal(t, employeeUpdated, employeeRead)

	// delete employee
	err = s.EmployeeDelete(correlationId, ctx, empNo)
	assert.Nil(t, err)

	//  read employee again
	employeeRead, err = s.EmployeeRead(correlationId, ctx, empNo)
	assert.NotNil(t, err)
	assert.Nil(t, employeeRead)

	// delete employee again
	err = s.EmployeeDelete(correlationId, ctx, empNo)
	assert.NotNil(t, err)
}

func (s *sqlTest) Configure(envs map[string]string) error {
	if err := s.Sql.Configure(envs); err != nil {
		return err
	}
	if err := s.Logger.Configure(envs); err != nil {
		return err
	}
	return nil
}

func testSql(t *testing.T) {
	const correlationId string = "test_sql"
	c := newSqlTest()

	err := c.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure sqlTest")
	}
	err = c.Open(correlationId)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open sqlTest")
	}
	defer func() {
		c.Close(correlationId)
	}()
	t.Run("Sql", c.testSql)
}

func TestSql(t *testing.T) {
	testSql(t)
}

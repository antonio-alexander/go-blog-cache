package sql_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"

	"github.com/stretchr/testify/assert"
)

var (
	envs = map[string]string{
		"DATABASE_HOST":          "localhost",
		"DATABASE_PORT":          "3306",
		"DATABASE_NAME":          "employees",
		"DATABASE_USER":          "mysql",
		"DATABASE_PASSWORD":      "mysql",
		"DATABASE_QUERY_TIMEOUT": "10",
		"DATABASE_PARSE_TIME":    "true",
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
	sql interface {
		internal.Opener
		internal.Configurer
	}
	sql.Sql
}

func newSqlTest() *sqlTest {
	sql := sql.NewMySql()
	return &sqlTest{
		sql: sql,
		Sql: sql,
	}
}

func (s *sqlTest) TestSql(t *testing.T) {
	// generate context
	ctx := context.TODO()

	// create employee
	birthDate, hireDate := time.Now().Unix(), time.Now().Unix()
	firstName := internal.GenerateId()[:14]
	lastName := internal.GenerateId()[:16]
	gender := "M"
	employeeCreated, err := s.EmployeeCreate(ctx, data.EmployeePartial{
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
		_ = s.EmployeeDelete(ctx, empNo)
	}(empNo)

	// read employee
	employeeRead, err := s.EmployeeRead(ctx, empNo)
	assert.Nil(t, err)
	assert.NotNil(t, employeeRead)
	assert.Equal(t, employeeCreated, employeeRead)

	// search employee
	employeesRead, err := s.EmployeesSearch(ctx,
		data.EmployeeSearch{EmpNos: []int64{empNo}})
	assert.Nil(t, err)
	assert.NotEmpty(t, employeesRead)
	assert.Len(t, employeesRead, 1)
	assert.Contains(t, employeesRead, employeeCreated)

	// update employee
	updatedFirstName := internal.GenerateId()[:14]
	updatedLastName := internal.GenerateId()[:16]
	employeeUpdated, err := s.EmployeeUpdate(ctx, empNo, data.EmployeePartial{
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
	employeeRead, err = s.EmployeeRead(ctx, empNo)
	assert.Nil(t, err)
	assert.NotNil(t, employeeRead)
	assert.Equal(t, employeeUpdated, employeeRead)

	// delete employee
	err = s.EmployeeDelete(ctx, empNo)
	assert.Nil(t, err)

	//  read employee again
	employeeRead, err = s.EmployeeRead(ctx, empNo)
	assert.NotNil(t, err)
	assert.Nil(t, employeeRead)

	// delete employee again
	err = s.EmployeeDelete(ctx, empNo)
	assert.NotNil(t, err)
}

func testSql(t *testing.T) {
	c := newSqlTest()

	ctx := context.TODO()
	err := c.sql.Configure(envs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to configure sqlTest")
	}
	err = c.sql.Open(ctx)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "unable to open sqlTest")
	}
	defer func() {
		_ = c.sql.Close(ctx)
	}()
	t.Run("Sql", c.TestSql)
}

func TestSql(t *testing.T) {
	testSql(t)
}

package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	_ "github.com/go-sql-driver/mysql" //import for driver support
)

const (
	databaseIsolation = sql.LevelSerializable
	tableDatabase     = "employees"
	tableEmployees    = "employees"
)

type Sql interface {
	EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeDelete(ctx context.Context, empNo int64) error
}

type mySql struct {
	sync.RWMutex
	config struct {
		Hostname       string        `json:"hostname"`
		Port           string        `json:"port"`
		Username       string        `json:"username"`
		Password       string        `json:"password"`
		Database       string        `json:"database"`
		ConnectTimeout time.Duration `json:"connect_timeout"`
		QueryTimeout   time.Duration `json:"query_timeout"`
		ParseTime      bool          `json:"parse_time"`
	}
	*sql.DB
	utilities.Logger
	opened bool
}

func NewMySql(parameters ...any) interface {
	internal.Configurer
	internal.Opener
	Sql
} {
	m := &mySql{}
	for _, parameter := range parameters {
		switch v := parameter.(type) {
		case utilities.Logger:
			m.Logger = v
		}
	}
	return m
}

func (s *mySql) Configure(envs map[string]string) error {
	if databaseHost := envs["DATABASE_HOST"]; databaseHost != "" {
		s.config.Hostname = databaseHost
	}
	if databasePort := envs["DATABASE_PORT"]; databasePort != "" {
		s.config.Port = databasePort
	}
	if database := envs["DATABASE_NAME"]; database != "" {
		s.config.Database = database
	}
	if username := envs["DATABASE_USER"]; username != "" {
		s.config.Username = username
	}
	if password := envs["DATABASE_PASSWORD"]; password != "" {
		s.config.Password = password
	}
	if _, ok := envs["DATABASE_QUERY_TIMEOUT"]; ok {
		i, _ := strconv.ParseInt(envs["DATABASE_QUERY_TIMEOUT"], 10, 64)
		s.config.QueryTimeout = time.Duration(i) * time.Second
	}
	if _, ok := envs["DATABASE_PARSE_TIME"]; ok {
		s.config.ParseTime, _ = strconv.ParseBool(envs["DATABASE_PARSE_TIME"])
	}
	return nil
}

func (s *mySql) Open(ctx context.Context) error {
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=%t",
		s.config.Username, s.config.Password, s.config.Hostname,
		s.config.Port, s.config.Database, s.config.ParseTime)
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	s.DB = db
	s.opened = true
	return nil
}

func (s *mySql) Close(ctx context.Context) error {
	if !s.opened {
		return nil
	}
	if err := s.DB.Close(); err != nil {
		s.Error(ctx, "error while closing sql: %s", err)
	}
	return nil
}

func (s *mySql) EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	var columns, values []string
	var args []any

	if employeePartial.BirthDate != nil {
		args = append(args, time.Unix(*employeePartial.BirthDate, 0))
		columns = append(columns, "birth_date")
		values = append(values, "?")
	}
	if employeePartial.FirstName != nil {
		args = append(args, employeePartial.FirstName)
		columns = append(columns, "first_name")
		values = append(values, "?")
	}
	if employeePartial.LastName != nil {
		args = append(args, employeePartial.LastName)
		columns = append(columns, "last_name")
		values = append(values, "?")
	}
	if employeePartial.Gender != nil {
		args = append(args, employeePartial.Gender)
		columns = append(columns, "gender")
		values = append(values, "?")
	}
	if employeePartial.HireDate != nil {
		args = append(args, time.Unix(*employeePartial.HireDate, 0))
		columns = append(columns, "hire_date")
		values = append(values, "?")
	}
	empNo, err := findEmpNo(ctx, s.DB)
	if err != nil {
		return nil, err
	}
	args = append(args, empNo)
	columns = append(columns, "emp_no")
	values = append(values, "?")
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableEmployees,
		strings.Join(columns, ","), strings.Join(values, ","))
	if _, err := s.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	return s.EmployeeRead(ctx, empNo)
}

func (s *mySql) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	query := fmt.Sprintf(`SELECT emp_no, birth_date, first_name, last_name,
		gender, hire_date FROM %s WHERE emp_no = ?;`,
		tableEmployees)
	row := s.QueryRowContext(ctx, query, empNo)
	employee, err := employeeScan(row.Scan)
	if err != nil {
		return nil, err
	}
	return employee, nil
}

func (s *mySql) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	var employees []*data.Employee

	criteria, args := employeeCriteria(search)
	query := fmt.Sprintf(`SELECT emp_no, birth_date, first_name, last_name,
		gender, hire_date FROM %s %s;`,
		tableEmployees, criteria)
	rows, err := s.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		employee, err := employeeScan(rows.Scan)
		if err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, nil
}

func (s *mySql) EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	var args []any
	var updates []string

	if employeePartial.BirthDate != nil {
		args = append(args, time.Unix(*employeePartial.BirthDate, 0))
		updates = append(updates, "birth_date = ?")
	}
	if employeePartial.FirstName != nil {
		args = append(args, employeePartial.FirstName)
		updates = append(updates, "first_name = ?")
	}
	if employeePartial.LastName != nil {
		args = append(args, employeePartial.LastName)
		updates = append(updates, "last_name = ?")
	}
	if employeePartial.Gender != nil {
		args = append(args, employeePartial.Gender)
		updates = append(updates, "gender =  ?")
	}
	if employeePartial.HireDate != nil {
		args = append(args, time.Unix(*employeePartial.HireDate, 0))
		updates = append(updates, "hire_date = ?")
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE emp_no = ?", tableEmployees,
		strings.Join(updates, ","))
	args = append(args, empNo)
	if _, err := s.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	return s.EmployeeRead(ctx, empNo)
}

func (s *mySql) EmployeeDelete(ctx context.Context, empNo int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE emp_no = ?;`,
		tableEmployees)
	result, err := s.ExecContext(ctx, query, empNo)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("employee not found")
	}
	return nil
}

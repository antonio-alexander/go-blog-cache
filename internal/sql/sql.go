package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/data"

	_ "github.com/go-sql-driver/mysql" //import for driver support
)

const (
	databaseIsolation = sql.LevelSerializable
	tableDatabase     = "employees"
	tableEmployees    = "employees"
)

type Sql struct {
	sync.RWMutex
	*sql.DB
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
}

func NewSql(parameters ...interface{}) *Sql {
	return &Sql{}
}

func (s *Sql) Configure(envs map[string]string) error {
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

func (s *Sql) Open() error {
	//EXAMPLE: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
	// user:password@tcp(localhost:5555)/dbname?charset=utf8
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
	return nil
}

func (s *Sql) Close() error {
	if err := s.DB.Close(); err != nil {
		fmt.Printf("error while closing sql: %s", err)
	}
	return nil
}

func (s *Sql) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
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

func (s *Sql) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
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

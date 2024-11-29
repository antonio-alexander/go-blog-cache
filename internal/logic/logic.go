package logic

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
)

type Logic struct {
	sync.RWMutex
	*sql.Sql
	cache  cache.Cache
	config struct {
		cacheEnabled bool
	}
}

func NewLogic(parameters ...interface{}) *Logic {
	l := &Logic{}
	for _, parameter := range parameters {
		switch v := parameter.(type) {
		case *sql.Sql:
			l.Sql = v
		case cache.Cache:
			l.cache = v
		}
	}
	return l
}

func (l *Logic) Configure(envs map[string]string) error {
	l.Lock()
	defer l.Unlock()

	if cacheEnabled, ok := envs["SERVICE_CACHE_ENABLED"]; ok {
		l.config.cacheEnabled, _ = strconv.ParseBool(cacheEnabled)
	}
	return nil
}

func (l *Logic) Open() error {
	l.Lock()
	defer l.Unlock()

	if l.config.cacheEnabled {
		fmt.Println("cache enabled")
	}
	return nil
}

func (l *Logic) Close() error {
	return nil
}

func (l *Logic) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	if l.config.cacheEnabled {
		employee, err := l.cache.EmployeeRead(ctx, empNo)
		if err == nil {
			return employee, nil
		}
		fmt.Printf("error while reading employee (%d) from cache: %s\n", empNo, err)
	}
	return l.Sql.EmployeeRead(ctx, empNo)
}

func (l *Logic) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	employees, err := l.Sql.EmployeesSearch(ctx, search)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(ctx, search, employees...); err != nil {
			fmt.Printf("error while writing employees to cache: %s", err)
		}
	}
	return employees, nil
}

func (l *Logic) EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	employee, err := l.Sql.EmployeeUpdate(ctx, empNo, employeePartial)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(ctx, empNo); err != nil {
			fmt.Printf("error while deleting employee (%d) from cache: %s", empNo, err)
		}
	}
	return employee, nil
}

func (l *Logic) EmployeeDelete(ctx context.Context, empNo int64) error {
	if err := l.Sql.EmployeeDelete(ctx, empNo); err != nil {
		return err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(ctx, empNo); err != nil {
			fmt.Printf("error while deleting employee (%d) from cache: %s", empNo, err)
		}
	}
	return nil
}

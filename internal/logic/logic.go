package logic

import (
	"context"
	"errors"
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
		cacheEnabled   bool
		mutateDisabled bool
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

	if cacheEnabled, ok := envs["LOGIC_CACHE_ENABLED"]; ok {
		l.config.cacheEnabled, _ = strconv.ParseBool(cacheEnabled)
	}
	if mutateDisabled, ok := envs["MUTATE_DISABLED"]; ok {
		l.config.mutateDisabled, _ = strconv.ParseBool(mutateDisabled)
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

func (l *Logic) EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	if l.config.mutateDisabled {
		return nil, errors.New("mutation disabled")
	}
	return l.Sql.EmployeeCreate(ctx, employeePartial)
}

func (l *Logic) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	if l.config.cacheEnabled {
		employee, err := l.cache.EmployeeRead(ctx, empNo)
		if err == nil {
			return employee, nil
		}
		fmt.Printf("error while reading employee (%d) from cache: %s\n", empNo, err)
	}
	employee, err := l.Sql.EmployeeRead(ctx, empNo)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(ctx, data.EmployeeSearch{}, employee); err != nil {
			fmt.Printf("error while writing employee (%d) to cache: %s\n", empNo, err)
		}
	}
	return employee, nil
}

func (l *Logic) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	if l.config.cacheEnabled {
		employees, err := l.cache.EmployeesRead(ctx, search)
		if err == nil {
			return employees, nil
		}
		fmt.Printf("error while reading employees from cache: %s\n", err)
	}
	employees, err := l.Sql.EmployeesSearch(ctx, search)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(ctx, search, employees...); err != nil {
			fmt.Printf("error while writing employees to cache: %s\n", err)
		}
	}
	return employees, nil
}

func (l *Logic) EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	if l.config.mutateDisabled {
		return nil, errors.New("mutation disabled")
	}
	employee, err := l.Sql.EmployeeUpdate(ctx, empNo, employeePartial)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(ctx, empNo); err != nil {
			fmt.Printf("error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return employee, nil
}

func (l *Logic) EmployeeDelete(ctx context.Context, empNo int64) error {
	if l.config.mutateDisabled {
		return errors.New("mutation disabled")
	}
	if err := l.Sql.EmployeeDelete(ctx, empNo); err != nil {
		return err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(ctx, empNo); err != nil {
			fmt.Printf("error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return nil
}

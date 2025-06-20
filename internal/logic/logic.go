package logic

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"
)

type logic struct {
	sync.RWMutex
	*sql.Sql
	utilities.Logger
	cache  cache.Cache
	config struct {
		cacheEnabled   bool
		mutateDisabled bool
	}
}

type Logic interface {
	Configure(envs map[string]string) error
	Open(correlationId string) error
	Close(correlationId string) error
	EmployeeCreate(correlationId string, ctx context.Context,
		employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeRead(correlationId string, ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesSearch(correlationId string, ctx context.Context,
		search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeeUpdate(correlationId string, ctx context.Context,
		empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeDelete(correlationId string, ctx context.Context, empNo int64) error
}

func NewLogic(parameters ...interface{}) Logic {
	l := &logic{}
	for _, parameter := range parameters {
		switch v := parameter.(type) {
		case *sql.Sql:
			l.Sql = v
		case cache.Cache:
			l.cache = v
		case utilities.Logger:
			l.Logger = v
		}
	}
	return l
}

func (l *logic) Configure(envs map[string]string) error {
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

func (l *logic) Open(correlationId string) error {
	l.Lock()
	defer l.Unlock()

	if l.config.cacheEnabled {
		l.Debug(correlationId, "logic cache enabled")
	}
	if l.config.mutateDisabled {
		l.Debug(correlationId, "logic mutation disbled")
	}
	return nil
}

func (l *logic) Close(correlationId string) error {
	return nil
}

func (l *logic) EmployeeCreate(correlationId string, ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	if l.config.mutateDisabled {
		return nil, errors.New("mutation disabled")
	}
	return l.Sql.EmployeeCreate(correlationId, ctx, employeePartial)
}

func (l *logic) EmployeeRead(correlationId string, ctx context.Context, empNo int64) (*data.Employee, error) {
	if l.config.cacheEnabled {
		employee, err := l.cache.EmployeeRead(correlationId, ctx, empNo)
		if err == nil {
			return employee, nil
		}
		l.Error(correlationId, "error while reading employee (%d) from cache: %s\n", empNo, err)
	}
	employee, err := l.Sql.EmployeeRead(correlationId, ctx, empNo)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(correlationId, ctx, data.EmployeeSearch{}, employee); err != nil {
			l.Error(correlationId, "error while writing employee (%d) to cache: %s\n", empNo, err)
		}
	}
	return employee, nil
}

func (l *logic) EmployeesSearch(correlationId string, ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	if l.config.cacheEnabled {
		employees, err := l.cache.EmployeesRead(correlationId, ctx, search)
		if err == nil {
			return employees, nil
		}
		l.Error(correlationId, "error while reading employees from cache: %s\n", err)
	}
	employees, err := l.Sql.EmployeesSearch(correlationId, ctx, search)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(correlationId, ctx, search, employees...); err != nil {
			l.Error(correlationId, "error while writing employees to cache: %s\n", err)
		}
	}
	return employees, nil
}

func (l *logic) EmployeeUpdate(correlationId string, ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	if l.config.mutateDisabled {
		return nil, errors.New("mutation disabled")
	}
	employee, err := l.Sql.EmployeeUpdate(correlationId, ctx, empNo, employeePartial)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(correlationId, ctx, empNo); err != nil {
			l.Error(correlationId, "error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return employee, nil
}

func (l *logic) EmployeeDelete(correlationId string, ctx context.Context, empNo int64) error {
	if l.config.mutateDisabled {
		return errors.New("mutation disabled")
	}
	if err := l.Sql.EmployeeDelete(correlationId, ctx, empNo); err != nil {
		return err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(correlationId, ctx, empNo); err != nil {
			l.Error(correlationId, "error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return nil
}

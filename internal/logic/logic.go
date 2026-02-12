package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/cenkalti/backoff/v5"
)

var ErrMutationDisabled = errors.New("mutation disabled")

type Logic interface {
	sql.Sql
}

type logic struct {
	sync.RWMutex
	config struct {
		cacheEnabled         bool
		cacheRetryInterval   int
		cacheMaxRetries      int
		cacheRetryExpBackoff bool
		mutateDisabled       bool
	}
	utilities.Logger
	utilities.Counter
	cache               cache.Cache
	sql                 sql.Sql
	backoffRetryOptions []backoff.RetryOption
}

func NewLogic(parameters ...any) interface {
	internal.Configurer
	internal.Opener
	Logic
} {
	l := &logic{}
	for _, parameter := range parameters {
		switch v := parameter.(type) {
		case sql.Sql:
			l.sql = v
		case cache.Cache:
			l.cache = v
		case utilities.Logger:
			l.Logger = v
		case utilities.Counter:
			l.Counter = v
		}
	}
	return l
}

func (l *logic) IncrementHit(item any) (hitCount int) {
	var key string

	if l.Counter == nil {
		return -1
	}
	switch v := item.(type) {
	default:
		return -1
	case string:
		key = fmt.Sprintf("employee_search_%s", v)
	case int64:
		key = fmt.Sprintf("employee_%d", v)
	}
	return l.Counter.IncrementHit(key)
}

func (l *logic) IncrementMiss(item any) (hitCount int) {
	var key string

	if l.Counter == nil {
		return -1
	}
	switch v := item.(type) {
	default:
		return -1
	case string:
		key = fmt.Sprintf("employee_search_%s", v)
	case int64:
		key = fmt.Sprintf("employee_%d", v)
	}
	return l.Counter.IncrementMiss(key)
}

func (l *logic) Configure(envs map[string]string) error {
	l.Lock()
	defer l.Unlock()

	l.config.cacheRetryInterval = 1
	l.config.cacheMaxRetries = 2
	l.config.cacheRetryExpBackoff = true
	if cacheEnabled, ok := envs["LOGIC_CACHE_ENABLED"]; ok {
		l.config.cacheEnabled, _ = strconv.ParseBool(cacheEnabled)
	}
	if mutateDisabled, ok := envs["MUTATE_DISABLED"]; ok {
		l.config.mutateDisabled, _ = strconv.ParseBool(mutateDisabled)
	}
	if cacheRetryInterval, ok := envs["CACHE_RETRY_INTERVAL"]; ok {
		l.config.cacheRetryInterval, _ = strconv.Atoi(cacheRetryInterval)
	}
	if cacheMaxRetries, ok := envs["CACHE_MAX_RETRIES"]; ok {
		l.config.cacheMaxRetries, _ = strconv.Atoi(cacheMaxRetries)
	}
	if cacheRetryExpBackoff, ok := envs["CACHE_RETRY_EXP_BACKOFF"]; ok {
		l.config.cacheRetryExpBackoff, _ = strconv.ParseBool(cacheRetryExpBackoff)
	}
	return nil
}

func (l *logic) Open(ctx context.Context) error {
	l.Lock()
	defer l.Unlock()

	if l.config.cacheEnabled && l.cache == nil {
		return errors.New("cache enabled, but no cache set/configured")
	}
	if l.config.cacheEnabled {
		l.Info(ctx, "cache enabled")
	}
	l.backoffRetryOptions = []backoff.RetryOption{
		backoff.WithMaxTries(uint(l.config.cacheMaxRetries)),
	}
	if l.config.cacheRetryExpBackoff {
		l.backoffRetryOptions = append(l.backoffRetryOptions,
			backoff.WithBackOff(backoff.NewExponentialBackOff()))
	}
	return nil
}

func (l *logic) Close(ctx context.Context) error {
	return nil
}

func (l *logic) EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	if l.config.mutateDisabled {
		return nil, ErrMutationDisabled
	}
	return l.sql.EmployeeCreate(ctx, employeePartial)
}

func (l *logic) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	if l.config.cacheEnabled {
		employee, err := backoff.Retry(ctx, func() (*data.Employee, error) {
			employee, err := l.cache.EmployeeRead(ctx, empNo)
			if err != nil {
				switch {
				default:
					return nil, backoff.Permanent(err)
				case errors.Is(err, cache.ErrEmployeeNotCached),
					errors.Is(err, cache.ErrEmployeeReadAlreadySet):
					l.Trace(ctx, "cache miss (retry) for employee (%d): %s", empNo, err)
					return nil, backoff.RetryAfter(l.config.cacheRetryInterval)
				}
			}
			return employee, nil
		}, l.backoffRetryOptions...)
		if err == nil {
			l.Trace(ctx, "employee (%d) read cache hit", empNo)
			l.IncrementHit(empNo)
			return employee, nil
		}
		l.Trace(ctx, "employee (%d) read cache miss", empNo)
		l.IncrementMiss(empNo)
	}
	employee, err := l.sql.EmployeeRead(ctx, empNo)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(ctx, data.EmployeeSearch{}, employee); err != nil {
			l.Trace(ctx, "error while writing employee (%d) to cache: %s", empNo, err)
		}
	}
	return employee, nil
}

func (l *logic) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	var searchKey string
	var err error

	if l.config.cacheEnabled {
		searchKey, err = search.ToKey()
		if err != nil {
			return nil, err
		}
		employees, err := backoff.Retry(ctx, func() ([]*data.Employee, error) {
			employees, err := l.cache.EmployeesRead(ctx, search)
			if err != nil {
				switch {
				default:
					return nil, backoff.Permanent(err)
				case errors.Is(err, cache.ErrEmployeeNotCached),
					errors.Is(err, cache.ErrEmployeeReadAlreadySet):
					l.Trace(ctx, "search cache miss (retry): %s", err)
					return nil, backoff.RetryAfter(l.config.cacheRetryInterval)
				}
			}
			return employees, nil
		}, l.backoffRetryOptions...)
		if err == nil {
			l.Trace(ctx, "employee  search (%s) read cache hit", searchKey)
			l.IncrementHit(searchKey)
			return employees, nil
		}
		l.Trace(ctx, "employee  search (%s) read cache miss", searchKey)
		l.IncrementMiss(searchKey)
	}
	employees, err := l.sql.EmployeesSearch(ctx, search)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesWrite(ctx, search, employees...); err != nil {
			l.Trace(ctx, "error while writing employees (%s) to cache: %s", searchKey, err)
		}
	}
	return employees, nil
}

func (l *logic) EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	if l.config.mutateDisabled {
		return nil, ErrMutationDisabled
	}
	employee, err := l.sql.EmployeeUpdate(ctx, empNo, employeePartial)
	if err != nil {
		return nil, err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(ctx, empNo); err != nil {
			l.Trace(ctx, "error while deleting employee (%d) from cache: %s", empNo, err)
		} else {
			l.Trace(ctx, "cache invalidated: %d", empNo)
		}
	}
	return employee, nil
}

func (l *logic) EmployeeDelete(ctx context.Context, empNo int64) error {
	if l.config.mutateDisabled {
		return ErrMutationDisabled
	}
	if err := l.sql.EmployeeDelete(ctx, empNo); err != nil {
		return err
	}
	if l.config.cacheEnabled {
		if err := l.cache.EmployeesDelete(ctx, empNo); err != nil {
			l.Trace(ctx, "error while deleting employee (%d) from cache: %s", empNo, err)
		}
	}
	return nil
}

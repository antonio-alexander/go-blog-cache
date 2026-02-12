package cache

import (
	"context"
	"fmt"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

var (
	ErrEmployeeNotCached            = data.NewNotCachedError("employee not cached")
	ErrEmployeeNotFoundCached       = data.NewNotCachedError("employee not found; cached")
	ErrEmployeeReadSet              = data.NewNotCachedRetryError("employee not cached, read set")
	ErrEmployeeReadAlreadySet       = data.NewNotCachedRetryError("employee not cached, read already set")
	ErrEmployeeSearchNotCached      = data.NewNotCachedError("employee search not cached")
	ErrEmployeeSearchNotFoundCached = data.NewNotCachedError("employee search not found; cached")
	ErrEmployeesSearchSet           = data.NewNotCachedRetryError("employees search not cached, read set")
	ErrEmployeesSearchAlreadySet    = data.NewNotCachedRetryError("employees search not cached, read already set")
	ErrSleepNotCached               = data.NewNotCachedError("sleep not cached")
	ErrSleepNotFoundCached          = data.NewNotCachedError("sleep not found; cached")
	ErrSleepReadSet                 = data.NewNotCachedRetryError("sleep not cached, read set")
	ErrSleepReadAlreadySet          = data.NewNotCachedRetryError("sleep not cached, read already set")
)

func ErrSearchKey(err error) error {
	return data.NewError(fmt.Errorf("error while creating search key: %w", err))
}

type Cache interface {
	EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error
	EmployeesDelete(ctx context.Context, empNos ...int64) error
	EmployeesNotFoundWrite(ctx context.Context, search data.EmployeeSearch, empNos ...int64) error

	SleepRead(ctx context.Context, sleepId string) (*data.Sleep, error)
	SleepWrite(ctx context.Context, sleep *data.Sleep) error
	SleepsDelete(ctx context.Context, sleepIds ...string) error
}

func copyEmployee(e *data.Employee) *data.Employee {
	employee := &data.Employee{}
	*employee = *e
	return employee
}

func copySleep(s *data.Sleep) *data.Sleep {
	sleep := &data.Sleep{}
	*sleep = *s
	return sleep
}

package cache

import (
	"context"
	"fmt"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

var (
	ErrEmployeeNotCached            = data.NewNotCachedError("employee not cached")
	ErrEmployeeSearchNotCached      = data.NewNotCachedError("employee search not cached")
	ErrEmployeeNotFoundCached       = data.NewNotCachedError("employee not found; cached")
	ErrEmployeeSearchNotFoundCached = data.NewNotCachedError("employee search not found; cached")
	ErrEmployeeReadSet              = data.NewNotCachedRetryError("employee not cached, read set")
	ErrEmployeeReadAlreadySet       = data.NewNotCachedRetryError("employee not cached, read already set")
	ErrEmployeesSearchSet           = data.NewNotCachedRetryError("employees search not cached, read set")
	ErrEmployeesSearchAlreadySet    = data.NewNotCachedRetryError("employees search not cached, read already set")
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
}

func copyEmployee(e *data.Employee) *data.Employee {
	employee := &data.Employee{}
	*employee = *e
	return employee
}

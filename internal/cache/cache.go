package cache

import (
	"context"
	"errors"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

var (
	ErrEmployeeNotCached         = errors.New("employee not cached")
	ErrEmployeeSearchNotCached   = errors.New("employee search not cached")
	ErrEmployeeReadSet           = errors.New("employee not cached, read set")
	ErrEmployeeReadAlreadySet    = errors.New("employee not cached, read already set")
	ErrEmployeesSearchSet        = errors.New("employees search not cached, read set")
	ErrEmployeesSearchAlreadySet = errors.New("employees search not cached, read already set")
)

type Cache interface {
	EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error
	EmployeesDelete(ctx context.Context, empNos ...int64) error
}

func copyEmployee(e *data.Employee) *data.Employee {
	employee := &data.Employee{}
	*employee = *e
	return employee
}

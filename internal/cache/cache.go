package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

func copyEmployee(e *data.Employee) *data.Employee {
	employee := &data.Employee{}
	*employee = *e
	return employee
}

func searchToKey(search data.EmployeeSearch) (string, error) {
	bytes, err := json.Marshal(search)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:]), nil
}

type Cache interface {
	Configure(envs map[string]string) error
	Open() error
	Close() error
	Clear(ctx context.Context) error
	EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeesWrite(ctx context.Context, search data.EmployeeSearch, es ...*data.Employee) error
	EmployeesDelete(ctx context.Context, empNos ...int64) error
}

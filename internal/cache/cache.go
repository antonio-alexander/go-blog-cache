package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"

	"github.com/antonio-alexander/go-blog-big-data/internal/data"
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
	Open(correlationId string) error
	Close(correlationId string) error
	Clear(correlationId string, ctx context.Context) error
	EmployeeRead(correlationId string, ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesRead(correlationId string, ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeesWrite(correlationId string, ctx context.Context, search data.EmployeeSearch, es ...*data.Employee) error
	EmployeesDelete(correlationId string, ctx context.Context, empNos ...int64) error
}

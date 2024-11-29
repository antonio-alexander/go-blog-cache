package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

type memoryCache struct {
	sync.RWMutex
	employees map[int64]*data.Employee      //map[emp_no]employee
	searches  map[string]map[int64]struct{} //map[search][emp_no]
}

func NewMemory(parameters ...interface{}) Cache {
	return &memoryCache{
		employees: make(map[int64]*data.Employee),
		searches:  make(map[string]map[int64]struct{}),
	}
}

func (c *memoryCache) Configure(envs map[string]string) error {
	return nil
}

func (c *memoryCache) Open() error {
	c.Lock()
	defer c.Unlock()

	return nil
}

func (c *memoryCache) Close() error {
	c.Lock()
	defer c.Unlock()

	return nil
}

func (c *memoryCache) Clear(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	//clear cache
	c.employees = make(map[int64]*data.Employee)
	c.searches = make(map[string]map[int64]struct{})
	return nil
}

func (c *memoryCache) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	employee, ok := c.employees[empNo]
	if !ok {
		return nil, errors.New("employee not found")
	}
	return copyEmployee(employee), nil
}

func (c *memoryCache) EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	searchKey, err := searchToKey(search)
	if err != nil {
		return nil, err
	}
	searches, ok := c.searches[searchKey]
	if !ok {
		return nil, errors.New("search not cached")
	}
	employees := make([]*data.Employee, 0, len(searches))
	for empNo := range searches {
		e, ok := c.employees[empNo]
		if !ok {
			continue
		}
		employees = append(employees, copyEmployee(e))
	}
	return employees, nil
}

func (c *memoryCache) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, es ...*data.Employee) error {
	c.Lock()
	defer c.Unlock()

	searchKey, err := searchToKey(search)
	if err != nil {
		fmt.Printf("error while creating search key: %s\n", err)
	}
	if _, ok := c.searches[searchKey]; !ok {
		c.searches[searchKey] = make(map[int64]struct{})
	}
	for _, e := range es {
		employee := copyEmployee(e)
		c.employees[employee.EmpNo] = employee
		c.searches[searchKey][employee.EmpNo] = struct{}{}
	}
	return nil
}

func (c *memoryCache) EmployeesDelete(ctx context.Context, empNos ...int64) error {
	c.Lock()
	defer c.Unlock()

	for _, empNo := range empNos {
		delete(c.employees, empNo)
	}
	return nil
}

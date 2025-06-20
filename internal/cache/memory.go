package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/antonio-alexander/go-blog-big-data/internal/data"
	"github.com/antonio-alexander/go-blog-big-data/internal/utilities"
)

type memoryCache struct {
	sync.RWMutex
	cacheCounter utilities.CacheCounter
	logger       utilities.Logger
	employees    map[int64]*data.Employee      //map[emp_no]employee
	searches     map[string]map[int64]struct{} //map[search][emp_no]
	configured   bool
}

func NewMemory(parameters ...interface{}) Cache {
	c := &memoryCache{}
	for _, p := range parameters {
		switch p := p.(type) {
		case utilities.CacheCounter:
			c.cacheCounter = p
		case utilities.Logger:
			c.logger = p
		}
	}
	return c
}

func (c *memoryCache) Error(correlationId, format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Error(correlationId, format, v...)
	}
}

func (c *memoryCache) Trace(correlationId, format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Trace(correlationId, format, v...)
	}
}

func (c *memoryCache) IncrementHit(item any) (hitCount int) {
	if c.cacheCounter != nil {
		switch v := item.(type) {
		case string:
			return c.cacheCounter.IncrementHit(fmt.Sprintf("employee_search_%s", v))
		case int64:
			return c.cacheCounter.IncrementHit(fmt.Sprintf("employee_%d", v))
		}
	}
	return -1
}

func (c *memoryCache) IncrementMiss(item any) (hitCount int) {
	if c.cacheCounter != nil {
		switch v := item.(type) {
		case string:
			return c.cacheCounter.IncrementMiss(fmt.Sprintf("employee_search_%s", v))
		case int64:
			return c.cacheCounter.IncrementMiss(fmt.Sprintf("employee_%d", v))
		}
	}
	return -1
}

func (c *memoryCache) Configure(envs map[string]string) error {
	c.configured = true
	return nil
}

func (c *memoryCache) Open(correlationId string) error {
	c.Lock()
	defer c.Unlock()

	c.employees = make(map[int64]*data.Employee)
	c.searches = make(map[string]map[int64]struct{})
	return nil
}

func (c *memoryCache) Close(correlationId string) error {
	c.Lock()
	defer c.Unlock()

	c.employees = nil
	c.searches = nil
	return nil
}

func (c *memoryCache) Clear(correlationId string, ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	//clear cache
	c.employees = make(map[int64]*data.Employee)
	c.searches = make(map[string]map[int64]struct{})
	return nil
}

func (c *memoryCache) EmployeeRead(correlationId string, ctx context.Context, empNo int64) (*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	employee, ok := c.employees[empNo]
	if !ok {
		c.IncrementMiss(empNo)
		c.Trace(correlationId, "cache miss for employee: %d", empNo)
		return nil, errors.New("employee not found")
	}
	c.IncrementHit(empNo)
	c.Trace(correlationId, "cache hit for employee: %d", empNo)
	return copyEmployee(employee), nil
}

func (c *memoryCache) EmployeesRead(correlationId string, ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	searchKey, err := searchToKey(search)
	if err != nil {
		return nil, err
	}
	searches, ok := c.searches[searchKey]
	if !ok {
		c.IncrementMiss(searchKey)
		c.Trace(correlationId, "cache miss for employee search: %s", searchKey)
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
	c.Trace(correlationId, "cache hit for employee search: %s", searchKey)
	c.IncrementHit(searchKey)
	return employees, nil
}

func (c *memoryCache) EmployeesWrite(correlationId string, ctx context.Context, search data.EmployeeSearch, es ...*data.Employee) error {
	c.Lock()
	defer c.Unlock()

	searchKey, err := searchToKey(search)
	if err != nil {
		c.Error(correlationId, "error while creating search key: %s", err)
		return err
	}
	if _, ok := c.searches[searchKey]; !ok {
		c.searches[searchKey] = make(map[int64]struct{})
		c.Trace(correlationId, "cached employees search: %s", searchKey)
	}
	for _, e := range es {
		employee := copyEmployee(e)
		c.employees[employee.EmpNo] = employee
		c.searches[searchKey][employee.EmpNo] = struct{}{}
		c.Trace(correlationId, "cached employee: %d", employee.EmpNo)
	}
	return nil
}

func (c *memoryCache) EmployeesDelete(correlationId string, ctx context.Context, empNos ...int64) error {
	c.Lock()
	defer c.Unlock()

	for _, empNo := range empNos {
		delete(c.employees, empNo)
		c.Trace(correlationId, "evicted cached employee: %d", empNo)
	}
	//TODO: invalidate the search key somehow?
	return nil
}

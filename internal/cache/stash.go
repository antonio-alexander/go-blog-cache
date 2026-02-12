package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/antonio-alexander/go-stash"
)

type stashCache struct {
	logger utilities.Logger
	stash  interface {
		stash.Configurer
		stash.Parameterizer
		stash.Initializer
		stash.Shutdowner
	}
	stash.Stasher
}

func NewStash(parameters ...any) interface {
	internal.Configurer
	internal.Opener
	internal.Clearer
	Cache
} {
	c := &stashCache{}
	for _, p := range parameters {
		switch p := p.(type) {
		case utilities.Logger:
			c.logger = p
		case interface {
			stash.Configurer
			stash.Parameterizer
			stash.Initializer
			stash.Shutdowner
			stash.Stasher
		}:
			c.stash = p
			c.Stasher = p
		}
	}
	if c.stash != nil {
		c.stash.SetParameters(parameters...)
	}
	return c
}

func (c *stashCache) Error(ctx context.Context, format string, v ...any) {
	if c.logger != nil {
		c.logger.Error(ctx, format, v...)
	}
}

func (c *stashCache) Trace(ctx context.Context, format string, v ...any) {
	if c.logger != nil {
		c.logger.Trace(ctx, format, v...)
	}
}

func (c *stashCache) Configure(envs map[string]string) error {
	if c.stash != nil {
		if err := c.stash.Configure(envs); err != nil {
			return err
		}
	}
	return nil
}

func (c *stashCache) Open(ctx context.Context) error {
	if c.stash != nil {
		return c.stash.Initialize()
	}
	return nil
}

func (c *stashCache) Close(ctx context.Context) error {
	if c.stash != nil {
		return c.stash.Shutdown()
	}
	return nil
}

func (c *stashCache) Clear(ctx context.Context) error {
	return c.Stasher.Clear()
}

func (c *stashCache) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	employee := &data.Employee{}
	if err := c.Stasher.Read(fmt.Sprint(empNo), employee); err != nil {
		c.Trace(ctx, "cache miss for employee: %d", empNo)
		c.Error(ctx, "error while reading employee: %s", err)
		return nil, errors.New("employee not found")
	}
	c.Trace(ctx, "cache hit for employee: %d", empNo)
	return employee, nil
}

func (c *stashCache) EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	//REVIEW: I don't think this is complete
	searchKey, err := search.ToKey()
	if err != nil {
		return nil, err
	}
	//REVIEW: should we pull the data out?
	if err := c.Stasher.Read(searchKey, &search); err != nil {
		c.Trace(ctx, "cache miss for employee search: %s", searchKey)
		return nil, errors.New("search not cached")
	}
	employees := make([]*data.Employee, 0, len(search.EmpNos))
	for _, empNo := range search.EmpNos {
		employee := &data.Employee{}
		if err := c.Stasher.Read(fmt.Sprint(empNo), employee); err != nil {
			//KIM: we don't want to fail half way, so any failure here
			// should return an error
			c.Trace(ctx, "cache miss for employee search: %s", searchKey)
			if err := c.Stasher.Delete(searchKey); err != nil {
				c.Error(ctx, "error while deleting searchkey (%s): %s",
					searchKey, err)
			}
			return nil, errors.New("employee not cached")
		}
		employees = append(employees, employee)
	}
	c.Trace(ctx, "cache hit for employee search: %s", searchKey)
	return employees, nil
}

func (c *stashCache) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	searchKey, err := search.ToKey()
	if err != nil {
		c.Error(ctx, "error while creating search key: %s", err)
		return err
	}
	if _, err := c.Stasher.Write(searchKey, &search); err != nil {
		c.Error(ctx, "error while writing search: %s", err)
		return err
	}
	c.Trace(ctx, "cached employees search: %s", searchKey)
	for _, employee := range employees {
		if _, err := c.Stasher.Write(fmt.Sprint(employee.EmpNo), employee); err != nil {
			// we don't care about the error here, but it does make the caching
			// incomplete
			c.Error(ctx, "error while writing employee (%d): %s", employee.EmpNo, err)
		}
		c.Trace(ctx, "cached employee: %d", employee.EmpNo)
	}
	return nil
}

func (c *stashCache) EmployeesDelete(ctx context.Context, empNos ...int64) error {
	for _, empNo := range empNos {
		if err := c.Stasher.Delete(fmt.Sprint(empNo)); err != nil {
			c.Error(ctx, "error while deleting employee")
			continue
		}
		c.Trace(ctx, "evicted cached employee: %d", empNo)
	}
	//KIM: even though we can't directly invalidate the search key, when you attempt to
	// use the search key and it's found, but not all the employees are, it's automatically
	// invalidated
	return nil
}

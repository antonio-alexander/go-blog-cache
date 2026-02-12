package cache

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"
)

type memoryCache struct {
	sync.RWMutex
	sync.WaitGroup
	employees  map[int64]*data.Employee      //map[emp_no]employee
	searches   map[string]map[int64]struct{} //map[search][emp_no]
	inProgress struct {
		sync.RWMutex
		employeeRead   map[int64]int64  //map[emp_no]int64
		employeeSearch map[string]int64 //map[search]int64
	}
	config struct {
		inProgressPruneInterval time.Duration
		inProgressTTL           time.Duration
		inProgressEnabled       bool
	}
	ctx       context.Context
	ctxCancel context.CancelFunc
	utilities.Logger
}

func NewMemory(parameters ...any) interface {
	internal.Configurer
	internal.Opener
	internal.Clearer
	Cache
} {
	c := &memoryCache{}
	for _, parameter := range parameters {
		switch p := parameter.(type) {
		case utilities.Logger:
			c.Logger = p
		}
	}
	return c
}

func (c *memoryCache) launchPruneSetRead() {
	started := make(chan struct{})
	c.Add(1)
	go func() {
		defer c.Done()

		pruneEmployeeReadFx := func() {
			c.inProgress.Lock()
			defer c.inProgress.Unlock()

			for key, t := range c.inProgress.employeeRead {
				if time.Since(time.Unix(0, t)) > c.config.inProgressTTL {
					delete(c.inProgress.employeeRead, key)
				}
			}
		}
		pruneEmployeeSearchFx := func() {
			c.inProgress.Lock()
			defer c.inProgress.Unlock()

			for key, t := range c.inProgress.employeeSearch {
				if time.Since(time.Unix(0, t)) > c.config.inProgressTTL {
					delete(c.inProgress.employeeSearch, key)
				}
			}
		}
		tPrune := time.NewTicker(c.config.inProgressPruneInterval)
		defer tPrune.Stop()
		close(started)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-tPrune.C:
				pruneEmployeeReadFx()
				pruneEmployeeSearchFx()
			}
		}
	}()
	<-started
}

func (c *memoryCache) Configure(envs map[string]string) error {
	if s, ok := envs["CACHE_PRUNE_INTERVAL"]; ok {
		inProgressPruneInterval, _ := strconv.Atoi(s)
		c.config.inProgressPruneInterval = time.Second * time.Duration(inProgressPruneInterval)
	}
	if c.config.inProgressPruneInterval <= 0 {
		c.config.inProgressPruneInterval = 10 * time.Second
	}
	if s, ok := envs["CACHE_SET_READ_TTL"]; ok {
		inProgressTTL, _ := strconv.Atoi(s)
		c.config.inProgressTTL = time.Second * time.Duration(inProgressTTL)
	}
	if inProgressEnabled, ok := envs["CACHE_ENABLE_IN_PROGRESS"]; ok {
		c.config.inProgressEnabled, _ = strconv.ParseBool(inProgressEnabled)
	}
	return nil
}

func (c *memoryCache) Open(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	c.employees = make(map[int64]*data.Employee)
	c.searches = make(map[string]map[int64]struct{})
	if c.config.inProgressEnabled {
		c.inProgress.employeeRead = make(map[int64]int64)
		c.inProgress.employeeSearch = make(map[string]int64)
		c.ctx, c.ctxCancel = context.WithCancel(context.Background())
		c.launchPruneSetRead()
	}
	return nil
}

func (c *memoryCache) Close(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	if c.config.inProgressEnabled {
		c.ctxCancel()
		c.Wait()
	}
	return nil
}

func (c *memoryCache) Clear(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	//clear cache
	c.employees = make(map[int64]*data.Employee)
	c.searches = make(map[string]map[int64]struct{})
	c.inProgress.employeeRead = make(map[int64]int64)
	c.inProgress.employeeSearch = make(map[string]int64)
	return nil
}

func (c *memoryCache) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	employee, ok := c.employees[empNo]
	if !ok {
		if !c.config.inProgressEnabled {
			return nil, ErrEmployeeNotCached
		}
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		if _, ok := c.inProgress.employeeRead[empNo]; ok {
			return nil, ErrEmployeeReadAlreadySet
		}
		c.inProgress.employeeRead[empNo] = time.Now().UnixNano()
		return nil, ErrEmployeeReadSet
	}
	return copyEmployee(employee), nil
}

func (c *memoryCache) EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	searchKey, err := search.ToKey()
	if err != nil {
		return nil, err
	}
	searches, ok := c.searches[searchKey]
	if !ok {
		if !c.config.inProgressEnabled {
			return nil, ErrEmployeeSearchNotCached
		}
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		if _, ok := c.inProgress.employeeSearch[searchKey]; ok {
			return nil, ErrEmployeesSearchAlreadySet
		}
		c.inProgress.employeeSearch[searchKey] = time.Now().UnixNano()
		return nil, ErrEmployeesSearchSet
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

func (c *memoryCache) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	c.Lock()
	defer c.Unlock()

	searchKey, err := search.ToKey()
	if err != nil {
		return fmt.Errorf("error while creating search key: %w\n", err)
	}
	if _, ok := c.searches[searchKey]; !ok {
		c.searches[searchKey] = make(map[int64]struct{})
	}
	for _, e := range employees {
		employee := copyEmployee(e)
		c.employees[employee.EmpNo] = employee
		c.searches[searchKey][employee.EmpNo] = struct{}{}
		if c.config.inProgressEnabled {
			delete(c.inProgress.employeeRead, e.EmpNo)
		}
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		delete(c.inProgress.employeeSearch, searchKey)
		for empNo := range c.searches[searchKey] {
			delete(c.inProgress.employeeRead, empNo)
		}
	}
	return nil
}

func (c *memoryCache) EmployeesDelete(ctx context.Context, empNos ...int64) error {
	c.Lock()
	defer c.Unlock()

	for _, empNo := range empNos {
		delete(c.employees, empNo)
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()

		for _, empNo := range empNos {
			delete(c.inProgress.employeeRead, empNo)
		}
	}
	return nil
}

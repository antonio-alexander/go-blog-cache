package cache

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"
)

type cacheEmployee struct {
	*data.Employee
	cachedAt int64
}

type cachedSleep struct {
	*data.Sleep
	cachedAt int64
}

type cachedEmployeeSearch struct {
	empNos   map[int64]struct{}
	cachedAt int64
}

type memoryCache struct {
	sync.RWMutex
	sync.WaitGroup
	employees        map[int64]cacheEmployee         //map[emp_no]cached_employee
	employeeSearches map[string]cachedEmployeeSearch //map[search]cached_employee_search
	sleeps           map[string]cachedSleep          //map[sleep_id]cached_sleep
	inProgress       struct {
		sync.RWMutex
		employeeRead   map[int64]int64  //map[emp_no]epoch
		employeeSearch map[string]int64 //map[search]epoch
		sleepRead      map[string]int64 //map[sleep_id]epoch
	}
	notFound struct {
		sync.RWMutex
		employeeNotFound       map[int64]int64  //map[emp_no]epoch
		employeeSearchNotFound map[string]int64 //map[search]epoch
	}
	config struct {
		inProgressTTL     time.Duration
		inProgressEnabled bool
		notFoundTTL       time.Duration
		notFoundEnabled   bool
		pruneInterval     time.Duration
		cacheTTL          time.Duration
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
					c.Trace(c.ctx, "pruned in progress (employee): %d", key)
				}
			}
		}
		pruneEmployeeSearchFx := func() {
			c.inProgress.Lock()
			defer c.inProgress.Unlock()

			for key, t := range c.inProgress.employeeSearch {
				if time.Since(time.Unix(0, t)) > c.config.inProgressTTL {
					delete(c.inProgress.employeeSearch, key)
					c.Trace(c.ctx, "pruned in progress (employee_search): %s", key)
				}
			}
		}
		pruneSleepReadFx := func() {
			c.inProgress.Lock()
			defer c.inProgress.Unlock()

			for key, t := range c.inProgress.sleepRead {
				if time.Since(time.Unix(0, t)) > c.config.inProgressTTL {
					delete(c.inProgress.sleepRead, key)
					c.Trace(c.ctx, "pruned in progress (sleep): %s", key)
				}
			}
		}
		tPrune := time.NewTicker(c.config.pruneInterval)
		defer tPrune.Stop()
		close(started)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-tPrune.C:
				pruneEmployeeReadFx()
				pruneEmployeeSearchFx()
				pruneSleepReadFx()
			}
		}
	}()
	<-started
}

func (c *memoryCache) launchPruneNotFound() {
	started := make(chan struct{})
	c.Add(1)
	go func() {
		defer c.Done()

		pruneEmployeeNotFoundFx := func() {
			c.notFound.Lock()
			defer c.notFound.Unlock()

			for key, t := range c.notFound.employeeNotFound {
				if time.Since(time.Unix(0, t)) > c.config.notFoundTTL {
					delete(c.notFound.employeeNotFound, key)
					c.Trace(c.ctx, "pruned not found (employee): %d", key)
				}
			}
		}
		pruneEmployeeNotFoundSearchFx := func() {
			c.notFound.Lock()
			defer c.notFound.Unlock()

			for key, t := range c.notFound.employeeSearchNotFound {
				if time.Since(time.Unix(0, t)) > c.config.notFoundTTL {
					delete(c.notFound.employeeSearchNotFound, key)
					c.Trace(c.ctx, "pruned not found (employee_search): %s", key)
				}
			}
		}
		tPrune := time.NewTicker(c.config.pruneInterval)
		defer tPrune.Stop()
		close(started)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-tPrune.C:
				pruneEmployeeNotFoundFx()
				pruneEmployeeNotFoundSearchFx()
			}
		}
	}()
	<-started
}

func (c *memoryCache) launchPruneCache() {
	started := make(chan struct{})
	c.Add(1)
	go func() {
		defer c.Done()

		pruneEmployeeReadFx := func() {
			c.Lock()
			defer c.Unlock()

			for key, t := range c.employees {
				if time.Since(time.Unix(0, t.cachedAt)) > c.config.cacheTTL {
					delete(c.employees, key)
					c.Trace(c.ctx, "pruned (employee): %d", key)
				}
			}
		}
		pruneEmployeeSearchFx := func() {
			c.Lock()
			defer c.Unlock()

			for key, t := range c.employeeSearches {
				if time.Since(time.Unix(0, t.cachedAt)) > c.config.cacheTTL {
					delete(c.employeeSearches, key)
					c.Trace(c.ctx, "pruned (employee_search): %s", key)
					for empNo := range t.empNos {
						delete(c.employees, empNo)
						c.Trace(c.ctx, "pruned (employee): %d", key)
					}
				}
			}
		}
		pruneSleepReadFx := func() {
			c.Lock()
			defer c.Unlock()

			for key, t := range c.sleeps {
				if time.Since(time.Unix(0, t.cachedAt)) > c.config.cacheTTL {
					delete(c.sleeps, key)
					c.Trace(c.ctx, "pruned (sleep): %s", key)
				}
			}
		}
		tPrune := time.NewTicker(c.config.pruneInterval)
		defer tPrune.Stop()
		close(started)
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-tPrune.C:
				pruneSleepReadFx()
				pruneEmployeeSearchFx() //searched before employees because of overlap
				pruneEmployeeReadFx()
			}
		}
	}()
	<-started
}

func (c *memoryCache) Configure(envs map[string]string) error {
	if s, ok := envs["CACHE_SET_READ_TTL"]; ok {
		inProgressTTL, _ := strconv.Atoi(s)
		c.config.inProgressTTL = time.Second * time.Duration(inProgressTTL)
	}
	if inProgressEnabled, ok := envs["CACHE_ENABLE_IN_PROGRESS"]; ok {
		c.config.inProgressEnabled, _ = strconv.ParseBool(inProgressEnabled)
	}
	if s, ok := envs["CACHE_NOT_FOUND_TTL"]; ok {
		notFoundTTL, _ := strconv.Atoi(s)
		c.config.notFoundTTL = time.Second * time.Duration(notFoundTTL)
	}
	if notFoundEnabled, ok := envs["CACHE_NOT_FOUND_ENABLED"]; ok {
		c.config.notFoundEnabled, _ = strconv.ParseBool(notFoundEnabled)
	}
	c.config.pruneInterval = time.Second
	if s, ok := envs["CACHE_PRUNE_INTERVAL"]; ok {
		i, _ := strconv.ParseInt(s, 10, 64)
		c.config.pruneInterval = time.Duration(i) * time.Second
	}
	c.config.cacheTTL = 5 * time.Second
	if s, ok := envs["CACHE_TTL"]; ok {
		i, _ := strconv.ParseInt(s, 10, 64)
		c.config.cacheTTL = time.Duration(i) * time.Second
	}
	return nil
}

func (c *memoryCache) Open(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	c.employees = make(map[int64]cacheEmployee)
	c.employeeSearches = make(map[string]cachedEmployeeSearch)
	c.sleeps = make(map[string]cachedSleep)
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
	c.launchPruneCache()
	if c.config.inProgressEnabled {
		c.inProgress.employeeRead = make(map[int64]int64)
		c.inProgress.employeeSearch = make(map[string]int64)
		c.inProgress.sleepRead = make(map[string]int64)
		c.launchPruneSetRead()
		c.Info(ctx, "cache: in progress enabled")
	}
	if c.config.notFoundEnabled {
		c.notFound.employeeNotFound = make(map[int64]int64)
		c.notFound.employeeSearchNotFound = make(map[string]int64)
		c.launchPruneNotFound()
		c.Info(ctx, "cache: not found enabled")
	}
	return nil
}

func (c *memoryCache) Close(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	c.ctxCancel()
	c.Wait()
	return nil
}

func (c *memoryCache) Clear(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	//clear cache
	c.employees = make(map[int64]cacheEmployee)
	c.employeeSearches = make(map[string]cachedEmployeeSearch)
	c.sleeps = make(map[string]cachedSleep)
	c.inProgress.employeeRead = make(map[int64]int64)
	c.inProgress.employeeSearch = make(map[string]int64)
	c.notFound.employeeNotFound = make(map[int64]int64)
	c.notFound.employeeSearchNotFound = make(map[string]int64)
	return nil
}

func (c *memoryCache) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	employee, ok := c.employees[empNo]
	if ok {
		return copyEmployee(employee.Employee), nil
	}
	if c.config.notFoundEnabled {
		c.notFound.RLock()
		defer c.notFound.RUnlock()
		if _, ok := c.notFound.employeeNotFound[empNo]; ok {
			return nil, ErrEmployeeNotFoundCached
		}
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		if _, ok := c.inProgress.employeeRead[empNo]; ok {
			return nil, ErrEmployeeReadAlreadySet
		}
		c.inProgress.employeeRead[empNo] = time.Now().UnixNano()
		return nil, ErrEmployeeReadSet
	}
	return nil, ErrEmployeeNotCached
}

func (c *memoryCache) EmployeesRead(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	c.RLock()
	defer c.RUnlock()

	searchKey, err := search.ToKey()
	if err != nil {
		return nil, err
	}
	employeeSearch, ok := c.employeeSearches[searchKey]
	if ok {
		employees := make([]*data.Employee, 0, len(employeeSearch.empNos))
		for empNo := range employeeSearch.empNos {
			e, ok := c.employees[empNo]
			if !ok {
				continue
			}
			employees = append(employees, copyEmployee(e.Employee))
		}
		return employees, nil
	}
	if c.config.notFoundEnabled {
		c.notFound.RLock()
		defer c.notFound.RUnlock()
		if _, ok := c.notFound.employeeSearchNotFound[searchKey]; ok {
			return nil, ErrEmployeeNotFoundCached
		}
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		if _, ok := c.inProgress.employeeSearch[searchKey]; ok {
			return nil, ErrEmployeesSearchAlreadySet
		}
		c.inProgress.employeeSearch[searchKey] = time.Now().UnixNano()
		return nil, ErrEmployeesSearchSet
	}
	return nil, ErrEmployeeSearchNotCached
}

func (c *memoryCache) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	c.Lock()
	defer c.Unlock()

	searchKey, err := search.ToKey()
	if err != nil {
		return ErrSearchKey(err)
	}
	cachedAt := time.Now().UnixNano()
	empNos := make(map[int64]struct{})
	for _, e := range employees {
		employee := copyEmployee(e)
		c.employees[employee.EmpNo] = cacheEmployee{
			Employee: employee,
			cachedAt: cachedAt,
		}
		empNos[employee.EmpNo] = struct{}{}
		if c.config.inProgressEnabled {
			delete(c.inProgress.employeeRead, e.EmpNo)
		}
	}
	c.employeeSearches[searchKey] = cachedEmployeeSearch{
		empNos:   empNos,
		cachedAt: cachedAt,
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		delete(c.inProgress.employeeSearch, searchKey)
		for empNo := range empNos {
			delete(c.inProgress.employeeRead, empNo)
		}
	}
	if c.config.notFoundEnabled {
		c.notFound.Lock()
		defer c.notFound.Unlock()
		delete(c.notFound.employeeSearchNotFound, searchKey)
		for empNo := range empNos {
			delete(c.notFound.employeeNotFound, empNo)
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
	if c.config.notFoundEnabled {
		c.notFound.Lock()
		defer c.notFound.Unlock()
		for _, empNo := range empNos {
			delete(c.notFound.employeeNotFound, empNo)
		}
	}
	return nil
}

func (c *memoryCache) EmployeesNotFoundWrite(ctx context.Context, search data.EmployeeSearch, empNos ...int64) error {
	c.Lock()
	defer c.Unlock()

	if !c.config.notFoundEnabled {
		return nil
	}
	searchKey, err := search.ToKey()
	if err != nil {
		return ErrSearchKey(err)
	}
	tNow := time.Now().UnixNano()
	c.notFound.Lock()
	defer c.notFound.Unlock()
	if _, ok := c.notFound.employeeSearchNotFound[searchKey]; !ok {
		c.notFound.employeeSearchNotFound[searchKey] = tNow
	}
	for _, empNo := range empNos {
		c.notFound.employeeNotFound[empNo] = tNow
	}
	return nil
}

func (c *memoryCache) SleepRead(ctx context.Context, sleepId string) (*data.Sleep, error) {
	c.RLock()
	defer c.RUnlock()

	sleep, ok := c.sleeps[sleepId]
	if ok {
		return copySleep(sleep.Sleep), nil
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		if _, ok := c.inProgress.sleepRead[sleepId]; ok {
			return nil, ErrSleepReadAlreadySet
		}
		c.inProgress.sleepRead[sleepId] = time.Now().UnixNano()
		return nil, ErrSleepReadSet
	}
	return nil, ErrSleepNotCached
}

func (c *memoryCache) SleepWrite(ctx context.Context, s *data.Sleep) error {
	c.Lock()
	defer c.Unlock()

	c.sleeps[s.Id] = cachedSleep{
		Sleep:    copySleep(s),
		cachedAt: time.Now().UnixNano(),
	}
	if c.config.inProgressEnabled {
		delete(c.inProgress.sleepRead, s.Id)
	}
	return nil
}

func (c *memoryCache) SleepsDelete(ctx context.Context, sleepIds ...string) error {
	c.Lock()
	defer c.Unlock()

	for _, sleepId := range sleepIds {
		delete(c.sleeps, sleepId)
	}
	if c.config.inProgressEnabled {
		c.inProgress.Lock()
		defer c.inProgress.Unlock()
		for _, sleepId := range sleepIds {
			delete(c.inProgress.sleepRead, sleepId)
		}
	}
	return nil
}

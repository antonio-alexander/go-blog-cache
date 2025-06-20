package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"
	"github.com/antonio-alexander/go-blog-cache/internal/service"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/pkg/errors"
)

var (
	Version   string
	GitCommit string
	GitBranch string
)

func init() {
	if Version = data.Version; Version == "" {
		Version = "<no_version_provided>"
	}
	if GitCommit = data.GitCommit; GitCommit == "" {
		GitCommit = "<no_git_commit>"
	}
	if GitBranch = data.GitBranch; GitBranch == "" {
		GitBranch = "<no_git_branch>"
	}
}

func main() {
	pwd, _ := os.Getwd()
	args := os.Args[1:]
	envs := make(map[string]string)
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)
	if err := Main(pwd, args, envs, osSignal); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}

func launchCounterDebug(correlationId string, wg *sync.WaitGroup, stopper chan os.Signal,
	logger utilities.Logger, counter utilities.CacheCounter, interval time.Duration) {
	started := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()

		tCheck := time.NewTicker(interval)
		defer tCheck.Stop()
		close(started)
		for {
			select {
			case <-stopper:
				return
			case <-tCheck.C:
				//read all counters
				counterHit, counterMiss := counter.ReadAll()
				for key, hit := range counterHit {
					logger.Trace(correlationId, "counter %v: hit (%d), miss (%d)",
						key, hit, counterMiss[key])
				}
			}
		}
	}()
	<-started
}

func launchTimerDebug(correlationId string, wg *sync.WaitGroup, stopper chan os.Signal,
	logger utilities.Logger, timer utilities.Timers, interval time.Duration) {
	started := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()

		tCheck := time.NewTicker(interval)
		defer tCheck.Stop()
		close(started)
		for {
			select {
			case <-stopper:
				return
			case <-tCheck.C:
				//read all timers
				totals, averages := timer.ReadAll()
				for group, total := range totals {
					logger.Trace(correlationId, "timer %s: total (%v), average (%v)",
						group, time.Duration(total)*time.Nanosecond,
						time.Duration(averages[group])*time.Nanosecond)
				}
			}
		}
	}()
	<-started
}

func Main(pwd string, args []string, envs map[string]string, osSignal chan os.Signal) error {
	const correlationId string = "go_blog_cache-service"
	var cacheCounter utilities.CacheCounter
	var employeeCache cache.Cache
	var wg sync.WaitGroup

	fmt.Printf("service: go-blog-cache v%s (%s) built from: %s\n",
		Version, GitCommit, GitBranch)

	// create logger, configure and open
	logger := utilities.NewLogger()
	if err := logger.Configure(envs); err != nil {
		return err
	}

	//create timers
	timers := utilities.NewTimers()

	//create sql, configure and open
	sql := sql.NewSql(logger)
	if err := sql.Configure(envs); err != nil {
		return err
	}
	if err := sql.Open(correlationId); err != nil {
		return err
	}
	defer func() {
		if err := sql.Close(correlationId); err != nil {
			fmt.Printf("error while closing sql: %s\n", err)
		}
	}()

	// create cache if configured
	cacheEnabled, _ := strconv.ParseBool(envs["LOGIC_CACHE_ENABLED"])
	cacheType := envs["LOGIC_CACHE_TYPE"]
	if cacheEnabled {
		// create cache counter
		cacheCounter = utilities.NewCacheCounter()

		switch cacheType {
		default:
			return errors.Errorf("unsupported cache type: %s", cacheType)
		case "redis":
			employeeCache = cache.NewRedis(logger, cacheCounter)
		case "memory":
			employeeCache = cache.NewMemory(logger, cacheCounter)
		}
		logger.Debug(correlationId, "configured cache: %s", cacheType)
		if err := employeeCache.Configure(envs); err != nil {
			return err
		}
		if err := employeeCache.Open(correlationId); err != nil {
			return err
		}
		defer func() {
			if err := employeeCache.Close(correlationId); err != nil {
				fmt.Printf("error while closing cache: %s\n", err)
			}
		}()
	}

	//create logic, configure and open
	logic := logic.NewLogic(sql, employeeCache, logger)
	if err := logic.Configure(envs); err != nil {
		return err
	}
	if err := logic.Open(correlationId); err != nil {
		return err
	}
	defer func() {
		if err := logic.Close(correlationId); err != nil {
			fmt.Printf("error while closing logic: %s\n", err)
		}
	}()

	//launch debugging processes if enabled
	if cacheCounterDebugEnabled, _ := strconv.ParseBool(envs["CACHE_COUNTER_DEBUG_ENABLED"]); cacheCounterDebugEnabled {
		interval, _ := strconv.Atoi(envs["CACHE_COUNTER_DEBUG_INTERVAL"])
		if interval > 0 {
			launchCounterDebug(correlationId, &wg, osSignal, logger, cacheCounter,
				time.Duration(interval)*time.Second)
		}
	}
	if timerDebugEnabled, _ := strconv.ParseBool(envs["TIMER_DEBUG_ENABLED"]); timerDebugEnabled {
		interval, _ := strconv.Atoi(envs["TIMER_DEBUG_INTERVAL"])
		if interval > 0 {
			launchTimerDebug(correlationId, &wg, osSignal, logger, timers,
				time.Duration(interval)*time.Second)
		}
	}

	//create service, configure and open
	service := service.NewService(logic, logger, timers)
	if err := service.Configure(envs); err != nil {
		return err
	}
	if err := service.Open(correlationId); err != nil {
		return err
	}
	<-osSignal
	service.Close(correlationId)
	wg.Wait()
	return nil
}

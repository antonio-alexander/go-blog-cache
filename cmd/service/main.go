package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"
	"github.com/antonio-alexander/go-blog-cache/internal/service"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/antonio-alexander/go-stash/memory"
	"github.com/antonio-alexander/go-stash/redis"
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

func createCache(envs map[string]string, parameters ...any) interface {
	internal.Configurer
	internal.Opener
	internal.Clearer
	cache.Cache
} {
	switch envs["CACHE_TYPE"] {
	default:
		return nil
	case "memory":
		return cache.NewMemory(parameters...)
	case "redis":
		return cache.NewRedis(parameters...)
	case "stash-memory":
		stash := memory.New()
		_ = stash.Configure(envs)
		parameters = append(parameters, stash)
		return cache.NewStash(parameters...)
	case "stash-redis":
		stash := redis.New()
		_ = stash.Configure(envs)
		parameters = append(parameters, stash)
		return cache.NewStash(parameters...)
	}
}

func Main(pwd string, args []string, envs map[string]string, osSignal chan os.Signal) error {
	var wg sync.WaitGroup

	//create context
	ctx, cancel := internal.LaunchContext(&wg, osSignal)
	defer cancel()

	// create utilities
	logger := utilities.NewLogger()
	_ = logger.Configure(envs)
	timers := utilities.NewTimers()
	counter := utilities.NewCounter()

	//print version info
	logger.Info(ctx, "server: go-blog-cache v%s (%s) built from: %s",
		Version, GitCommit, GitBranch)

	//create sql, configure and open
	sql := sql.NewMySql(logger)
	if err := sql.Configure(envs); err != nil {
		return err
	}
	if err := sql.Open(ctx); err != nil {
		return err
	}
	defer func() {
		if err := sql.Close(ctx); err != nil {
			logger.Error(context.Background(), "error while closing sql: %s", err)
		}
	}()

	// create cache
	cache := createCache(envs, logger)
	if cache != nil {
		if err := cache.Configure(envs); err != nil {
			return err
		}
		if err := cache.Open(ctx); err != nil {
			return err
		}
		defer func() {
			if err := cache.Close(context.Background()); err != nil {
				logger.Error(context.Background(), "error while closing cache: %s", err)
			}
		}()
	}

	//create logic, configure and open
	logic := logic.NewLogic(sql, logger, counter, cache)
	if err := logic.Configure(envs); err != nil {
		return err
	}
	if err := logic.Open(ctx); err != nil {
		return err
	}
	defer func() {
		if err := logic.Close(context.Background()); err != nil {
			logger.Error(context.Background(), "error while closing logic: %s", err)
		}
	}()

	//create service, configure and open
	service := service.NewService(logic, cache, logger, counter, timers)
	if err := service.Configure(envs); err != nil {
		return err
	}
	if err := service.Open(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	wg.Wait()
	service.Close(context.Background())
	return nil
}

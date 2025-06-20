package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/antonio-alexander/go-blog-big-data/internal/cache"
	"github.com/antonio-alexander/go-blog-big-data/internal/client"
	"github.com/antonio-alexander/go-blog-big-data/internal/data"
	"github.com/antonio-alexander/go-blog-big-data/internal/utilities"
	"github.com/antonio-alexander/go-stash"
	"github.com/antonio-alexander/go-stash/memory"
	"github.com/antonio-alexander/go-stash/redis"

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
	args := os.Args[1:]
	envs := make(map[string]string)
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)
	if err := Main(args, envs, osSignal); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}

func Main(args []string, envs map[string]string, osSignal chan (os.Signal)) error {
	const correlationId string = "go_blog_cache-client"
	var cacheCounter utilities.CacheCounter
	var employeeCache cache.Cache
	var stash interface {
		stash.Stasher
		stash.Configurer
		stash.Initializer
		stash.Shutdowner
		stash.Parameterizer
	}

	// create logger, configure and open
	logger := utilities.NewLogger()
	if err := logger.Configure(envs); err != nil {
		return err
	}

	logger.Info(correlationId, "client: go-blog-big-data v%s (%s) built from: %s",
		Version, GitCommit, GitBranch)

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
		case "stash-memory":
			stash = memory.New()
			stash.SetParameters(logger)
			employeeCache = cache.NewStash(logger, cacheCounter, stash)
		case "stash-redis":
			stash = redis.New()
			stash.SetParameters(logger)
			employeeCache = cache.NewStash(logger, cacheCounter, stash)
		}
		if stash != nil {
			if err := stash.Configure(envs); err != nil {
				return err
			}
		}
		if err := employeeCache.Configure(envs); err != nil {
			return err
		}
		if err := employeeCache.Open(correlationId); err != nil {
			return err
		}
		defer func() {
			if err := employeeCache.Close(correlationId); err != nil {
				logger.Error(correlationId, "error while closing cache: %s", err)
			}
		}()
	}

	//create client
	client := client.NewClient(employeeCache, logger)
	if err := client.Configure(envs); err != nil {
		return err
	}
	if err := client.Open(correlationId); err != nil {
		return err
	}
	defer func() {
		if err := client.Close(correlationId); err != nil {
			logger.Error(correlationId, "error while closing client: %s", err)
		}
	}()

	// execute command
	ctx, command := context.Background(), envs["COMMAND"]
	empNo, _ := strconv.ParseInt(envs["EMP_NO"], 10, 64)
	switch command {
	default:
		return errors.Errorf("unsupported command: %s", command)
	case "employee_read":
		employee, err := client.EmployeeRead(correlationId, ctx, empNo)
		if err != nil {
			return err
		}
		bytes, err := json.MarshalIndent(employee, "", " ")
		if err != nil {
			return err
		}
		fmt.Println(string(bytes))
	}
	return nil
}

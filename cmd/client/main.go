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

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/client"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
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

	fmt.Printf("client: go-blog-cache v%s (%s) built from: %s\n",
		Version, GitCommit, GitBranch)

	// create logger, configure and open
	logger := utilities.NewLogger()
	if err := logger.Configure(envs); err != nil {
		return err
	}

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
			fmt.Printf("error while closing client: %s\n", err)
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

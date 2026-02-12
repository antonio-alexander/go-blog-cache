package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/client"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

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

// determine hit/miss ratio with concurrent reads when
// invalidating the cache, possibly overall benchmark too
func scenarioStampedingHerd(ctx context.Context, envs map[string]string, logger utilities.Logger,
	clients ...client.Client) error {
	const correlationId string = "scenario_stampeding_herd"
	const minClients int = 2

	var readInterval time.Duration = time.Second
	var updateInterval time.Duration = 2 * time.Second
	var scenarioDuration time.Duration = 10 * time.Second
	var wg sync.WaitGroup

	if s := envs["SCENARIO_READ_INTERVAL"]; s != "" {
		i, _ := strconv.Atoi(s)
		readInterval = time.Duration(i) * time.Second
	}
	if s := envs["SCENARIO_UPDATE_INTERVAL"]; s != "" {
		i, _ := strconv.Atoi(s)
		updateInterval = time.Duration(i) * time.Second
	}
	if s := envs["SCENARIO_DURATION"]; s != "" {
		i, _ := strconv.Atoi(s)
		scenarioDuration = time.Duration(i) * time.Second
	}
	if len(clients) < minClients {
		return errors.New("not enough clients provided")
	}

	//generate context
	ctx = internal.CtxWithCorrelationId(ctx, correlationId)

	// create employee using the first client
	birthDate := time.Now().Unix()
	firstName := internal.GenerateId()[:14]
	lastName := internal.GenerateId()[:16]
	gender, hireDate := "M", time.Now().Unix()
	employeeCreated, err := clients[0].EmployeeCreate(ctx, data.EmployeePartial{
		BirthDate: &birthDate,
		FirstName: &firstName,
		LastName:  &lastName,
		Gender:    &gender,
		HireDate:  &hireDate,
	})
	if err != nil {
		return err
	}
	empNo := employeeCreated.EmpNo
	defer func(empNo int64) {
		_ = clients[0].EmployeeDelete(ctx, empNo)
		logger.Info(ctx, "deleted employee: %d", empNo)
	}(empNo)
	logger.Info(ctx, "created employee: %d", empNo)

	//generate start/stop channels
	start, stop := make(chan struct{}), make(chan struct{})

	//create writer go routine
	wg.Add(1)
	go func(ctx context.Context, client client.Client) {
		defer wg.Done()
		ctx = internal.CtxWithCorrelationId(ctx, correlationId)
		firstName := internal.GenerateId()[:14]
		lastName := internal.GenerateId()[:16]
		updateEmployeeFx := func(ctx context.Context) error {
			if _, err := client.EmployeeUpdate(ctx, empNo,
				data.EmployeePartial{
					FirstName: &firstName,
					LastName:  &lastName,
				}); err != nil {
				return err
			}
			return nil
		}
		tUpdate := time.NewTicker(updateInterval)
		defer tUpdate.Stop()
		<-start
		for {
			select {
			case <-stop:
				return
			case <-tUpdate.C:
				if err := updateEmployeeFx(ctx); err != nil {
					logger.Error(ctx, "error while updating employee: %s", err)
				}
			}
		}
	}(ctx, clients[0])

	//create reader go routines
	for i := 1; i < len(clients); i++ {
		wg.Add(1)
		go func(ctx context.Context, clientNumber int, client client.Client) {
			defer wg.Done()

			correlationId := fmt.Sprintf("scenario_stampeding_herd_%d", clientNumber)
			ctx = internal.CtxWithCorrelationId(ctx, correlationId)
			readEmployeeFx := func(ctx context.Context) error {
				if _, err := client.EmployeeRead(ctx, empNo); err != nil {
					return err
				}
				return nil
			}
			tRead := time.NewTicker(readInterval)
			defer tRead.Stop()
			<-start
			for {
				select {
				case <-stop:
					return
				case <-tRead.C:
					if err := readEmployeeFx(ctx); err != nil {
						logger.Error(ctx, "error while reading employee: %s", err)
					}
				}
			}
		}(ctx, i, clients[i])
	}

	//clear cache counters and start the go routines
	if err := clients[0].CacheClear(ctx); err != nil {
		return err
	}
	if err := clients[0].CacheCountersClear(ctx); err != nil {
		return err
	}
	close(start)

	//allow go routines to run
	<-time.After(scenarioDuration)

	//stop go routines
	close(stop)
	wg.Wait()

	//use initial client to get hit/miss ratios from server
	cacheCounters, err := clients[0].CacheCountersRead(ctx)
	if err != nil {
		return err
	}
	hit := cacheCounters.CounterHits[fmt.Sprintf("employee_%d", empNo)]
	miss := cacheCounters.CounterMisses[fmt.Sprintf("employee_%d", empNo)]
	total := hit + miss
	logger.Info(ctx, "cache hit miss ratio (%d/%d): %0.2f%%",
		hit, total, float64(hit)/float64(total)*100)

	return nil
}

func Main(args []string, envs map[string]string, osSignal chan (os.Signal)) error {
	var clients []client.Client
	var wg sync.WaitGroup

	//create context
	ctx, cancel := internal.LaunchContext(&wg, osSignal)
	defer cancel()

	// create logger
	logger := utilities.NewLogger()
	_ = logger.Configure(envs)

	//print version info
	logger.Info(ctx, "scenarios: go-blog-cache v%s (%s) built from: %s",
		Version, GitCommit, GitBranch)

	nClients, _ := strconv.Atoi(envs["N_CLIENTS"])
	for range nClients {
		//create cache
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
					logger.Error(ctx, "error while closing cache: %s", err)
				}
			}()
		}

		//create client
		client := client.NewClient(cache, logger)
		if err := client.Configure(envs); err != nil {
			return err
		}
		if err := client.Open(ctx); err != nil {
			return err
		}
		defer func() {
			if err := client.Close(context.Background()); err != nil {
				logger.Error(ctx, "error while closing client: %s", err)
			}
		}()
		clients = append(clients, client)
	}

	// execute scenario
	switch scenario := envs["SCENARIO"]; scenario {
	default:
		return errors.Errorf("unsupported scenario: %s", scenario)
	case "stampeding_herd":
		logger.Info(ctx, "executing %s scenario", scenario)
		if err := scenarioStampedingHerd(ctx, envs, logger, clients...); err != nil {
			logger.Error(ctx, "error while executing %s scenario: %s", scenario, err)
		}
	}
	cancel()
	wg.Wait()
	return nil
}

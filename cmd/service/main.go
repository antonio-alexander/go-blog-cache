package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"
	"github.com/antonio-alexander/go-blog-cache/internal/service"
	"github.com/antonio-alexander/go-blog-cache/internal/sql"
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

func Main(pwd string, args []string, envs map[string]string, osSignal chan os.Signal) error {
	fmt.Printf("service: go-blog-cache v%s (%s) built from: %s\n",
		Version, GitCommit, GitBranch)

	//create sql, configure and open
	sql := sql.NewSql()
	if err := sql.Configure(envs); err != nil {
		return err
	}
	if err := sql.Open(); err != nil {
		return err
	}
	defer func() {
		if err := sql.Close(); err != nil {
			fmt.Printf("error while closing sql: %s\n", err)
		}
	}()

	// create cache
	cache := cache.NewRedis()
	if err := cache.Configure(envs); err != nil {
		return err
	}
	if err := cache.Open(); err != nil {
		return err
	}
	defer func() {
		if err := cache.Close(); err != nil {
			fmt.Printf("error while closing cache: %s\n", err)
		}
	}()

	//create logic, configure and open
	logic := logic.NewLogic(sql)
	if err := logic.Configure(envs); err != nil {
		return err
	}
	if err := logic.Open(); err != nil {
		return err
	}
	defer func() {
		if err := logic.Close(); err != nil {
			fmt.Printf("error while closing logic: %s\n", err)
		}
	}()

	//create service, configure and open
	service := service.NewService(logic, cache)
	if err := service.Configure(envs); err != nil {
		return err
	}
	if err := service.Open(); err != nil {
		return err
	}
	<-osSignal
	service.Close()
	return nil
}

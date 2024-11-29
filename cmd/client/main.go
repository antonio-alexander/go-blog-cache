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
	fmt.Printf("client: go-blog-cache v%s (%s) built from: %s\n",
		Version, GitCommit, GitBranch)

	//create cache
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

	//create client
	client := client.NewClient(cache)
	if err := client.Configure(envs); err != nil {
		return err
	}
	if err := client.Open(); err != nil {
		return err
	}
	defer func() {
		if err := client.Close(); err != nil {
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
		employee, err := client.EmployeeRead(ctx, empNo)
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

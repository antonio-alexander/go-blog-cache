package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/cenkalti/backoff/v5"
	"github.com/pkg/errors"
)

type Client interface {
	EmployeeCreate(ctx context.Context,
		employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error)
	EmployeesSearch(ctx context.Context,
		search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeeUpdate(ctx context.Context, empNo int64,
		employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeDelete(ctx context.Context, empNo int64) error

	CacheClear(ctx context.Context) error
	CacheCountersRead(ctx context.Context) (*data.CacheCounters, error)
	CacheCountersClear(ctx context.Context) error

	TimersRead(ctx context.Context) (*data.Timers, error)
	TimersClear(ctx context.Context) error

	Sleep(ctx context.Context, duration time.Duration) error
}

type client struct {
	sync.RWMutex
	config struct {
		protocol        string
		address         string
		port            string
		timeout         int64
		sslCaFile       string
		sslCrtFile      string
		sslKeyFile      string
		cacheDisabled   bool
		maxRetries      int
		retryExpBackoff bool
	}
	address             string
	cache               cache.Cache
	ctx                 context.Context
	ctxCancel           context.CancelFunc
	opened              bool
	backoffRetryOptions []backoff.RetryOption
	utilities.Logger
	*http.Client
}

func NewClient(parameters ...any) interface {
	internal.Configurer
	internal.Opener
	Client
} {
	c := &client{Client: &http.Client{}}
	for _, parameter := range parameters {
		switch p := parameter.(type) {
		case cache.Cache:
			c.cache = p
		case utilities.Logger:
			c.Logger = p
		}
	}
	return c
}

func (c *client) doRequest(ctx context.Context, method, uri string, item any) ([]byte, error) {
	return backoff.Retry(ctx, func() ([]byte, error) {
		bytes, retryAfter, err := doRequest(ctx, c.Client, method, uri, item)
		if err != nil {
			switch {
			default:
				return nil, backoff.Permanent(err)
			case retryAfter > 0:
				return nil, backoff.RetryAfter(int(retryAfter))
			}
		}
		return bytes, nil
	}, c.backoffRetryOptions...)
}

func (c *client) Configure(envs map[string]string) error {
	if address, ok := envs["CLIENT_ADDRESS"]; ok {
		c.config.address = address
	}
	if port, ok := envs["CLIENT_PORT"]; ok {
		c.config.port = port
	}
	if protocol, ok := envs["CLIENT_PROTOCOL"]; ok {
		c.config.protocol = protocol
	}
	if timeout, ok := envs["CLIENT_TIMEOUT"]; ok {
		i, err := strconv.ParseInt(timeout, 10, 64)
		if err != nil {
			return err
		}
		c.config.timeout = i
	}
	if sslCaFile, ok := envs["SSL_CA_FILE"]; ok {
		c.config.sslCaFile = sslCaFile
	}
	if sslKeyFile, ok := envs["SSL_KEY_FILE"]; ok {
		c.config.sslKeyFile = sslKeyFile
	}
	if sslCrtFile, ok := envs["SSL_CRT_FILE"]; ok {
		c.config.sslCrtFile = sslCrtFile
	}
	if s, ok := envs["CACHE_DISABLED"]; ok {
		cacheDisabled, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		c.config.cacheDisabled = cacheDisabled
	}
	if s, ok := envs["CLIENT_MAX_RETRIES"]; ok {
		maxRetries, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		c.config.maxRetries = maxRetries
	}
	if c.config.maxRetries < 1 {
		c.config.maxRetries = 1
	}
	return nil
}

func (c *client) Open(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	if c.opened {
		return nil
	}
	switch c.config.protocol {
	default:
		return errors.Errorf("unsupported protocol: %s", c.config.protocol)
	case "http":
	case "https":
		caCertPool, err := getCaCert(c.config.sslCaFile)
		if err != nil {
			return errors.Wrap(err, "unable to read ca cert")
		}
		certificate, err := getCertificate(c.config.sslCrtFile, c.config.sslKeyFile)
		if err != nil {
			return errors.Wrap(err, "unable to read client cert")
		}
		c.Client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				// TLS versions below 1.2 are considered insecure
				// see https://www.rfc-editor.org/rfc/rfc7525.txt for details
				MinVersion:   tls.VersionTLS12,
				RootCAs:      caCertPool,
				Certificates: []tls.Certificate{certificate},
			},
		}
	}
	c.Client.Timeout = time.Duration(c.config.timeout) * time.Second
	c.address = fmt.Sprintf("%s://%s", c.config.protocol,
		strings.TrimSuffix(net.JoinHostPort(c.config.address, c.config.port), ":"))
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
	c.backoffRetryOptions = []backoff.RetryOption{
		backoff.WithMaxTries(uint(c.config.maxRetries)),
	}
	if c.config.retryExpBackoff {
		c.backoffRetryOptions = append(c.backoffRetryOptions,
			backoff.WithBackOff(backoff.NewExponentialBackOff()))
	}
	if c.config.cacheDisabled {
		c.Info(ctx, "client: cache disabled")
	}
	c.opened = true
	return nil
}

func (c *client) Close(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	if !c.opened {
		return nil
	}
	c.ctxCancel()
	c.opened = false
	return nil
}

func (c *client) EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	bytes, err := json.Marshal(&data.Request{
		EmployeePartial: employeePartial})
	if err != nil {
		return nil, err
	}
	uri := c.address + data.RouteEmployees
	bytes, err = c.doRequest(ctx, http.MethodPut, uri, bytes)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	return response.Employee, nil
}

func (c *client) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	if !c.config.cacheDisabled {
		employee, err := c.cache.EmployeeRead(ctx, empNo)
		if err == nil {
			return employee, nil
		}
		c.Error(ctx, "error while reading employee (%d) from cache: %s", empNo, err)
	}
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	bytes, err := c.doRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesWrite(ctx, data.EmployeeSearch{}, response.Employee); err != nil {
			c.Error(ctx, "error while writing employee (%d) to cache: %s", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *client) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	var response data.Response

	if !c.config.cacheDisabled {
		employees, err := c.cache.EmployeesRead(ctx, search)
		if err == nil {
			return employees, nil
		}
		c.Error(ctx, "error while reading employees from cache: %s", err)
	}
	params := search.ToParams()
	uri := c.address + data.RouteEmployeesSearch
	bytes, err := c.doRequest(ctx, http.MethodGet, uri, params)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesWrite(ctx, search, response.Employees...); err != nil {
			c.Error(ctx, "error while writing employees to cache: %s", err)
		}
	}
	return response.Employees, nil
}

func (c *client) EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	bytes, err := json.Marshal(&data.Request{EmployeePartial: employeePartial})
	if err != nil {
		return nil, err
	}
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	bytes, err = c.doRequest(ctx, http.MethodPost, uri, bytes)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(ctx, empNo); err != nil {
			c.Error(ctx, "error while deleting employee (%d) from cache: %s", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *client) EmployeeDelete(ctx context.Context, empNo int64) error {
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	if _, err := c.doRequest(ctx, http.MethodDelete, uri, nil); err != nil {
		return err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(ctx, empNo); err != nil {
			c.Error(ctx, "error while deleting employee (%d) from cache: %s", empNo, err)
		}
	}
	return nil
}

func (c *client) CacheClear(ctx context.Context) error {
	uri := fmt.Sprintf(c.address + data.RouteCache)
	if _, err := c.doRequest(ctx, http.MethodDelete, uri, nil); err != nil {
		return err
	}
	return nil
}

func (c *client) CacheCountersRead(ctx context.Context) (*data.CacheCounters, error) {
	uri := fmt.Sprintf(c.address + data.RouteCacheCounters)
	bytes, err := c.doRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	response := &data.CacheCounters{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *client) CacheCountersClear(ctx context.Context) error {
	uri := fmt.Sprintf(c.address + data.RouteCacheCounters)
	if _, err := c.doRequest(ctx, http.MethodDelete, uri, nil); err != nil {
		return err
	}
	return nil
}

func (c *client) TimersRead(ctx context.Context) (*data.Timers, error) {
	uri := fmt.Sprintf(c.address + data.RouteTimers)
	bytes, err := c.doRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	response := &data.Timers{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *client) TimersClear(ctx context.Context) error {
	uri := fmt.Sprintf(c.address + data.RouteTimers)
	if _, err := c.doRequest(ctx, http.MethodDelete, uri, nil); err != nil {
		return err
	}
	return nil
}

func (c *client) Sleep(ctx context.Context, duration time.Duration) error {
	uri := fmt.Sprintf(c.address + data.RouteSleep)
	bytes, err := json.Marshal(&data.Request{
		SleepDuration: duration,
	})
	if err != nil {
		return err
	}
	if _, err := c.doRequest(ctx, http.MethodPost, uri, bytes); err != nil {
		return err
	}
	return nil
}

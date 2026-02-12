package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal"
	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

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
}

type client struct {
	sync.RWMutex
	config struct {
		protocol      string
		address       string
		port          string
		timeout       int64
		sslCaFile     string
		sslCrtFile    string
		sslKeyFile    string
		cacheDisabled bool
	}
	address string
	cache   cache.Cache
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

func (c *client) doRequest(ctx context.Context, uri, method string, item any) ([]byte, error) {
	var contentLength int
	var contentType string
	var body io.Reader

	switch d := item.(type) {
	case []byte:
		body = bytes.NewBuffer(d)
		contentLength = len(d)
		contentType = "application/json"
	case url.Values:
		switch method {
		default:
			uri = uri + "?" + d.Encode()
		case http.MethodPut, http.MethodPost, http.MethodPatch:
			body = strings.NewReader(d.Encode())
			contentType = "application/x-www-form-urlencoded"
			contentLength = len(d.Encode())
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", contentType)
	request.Header.Add("Content-Length", strconv.Itoa(contentLength))
	request.Header.Add("Correlation-Id", internal.CorrelationIdFromCtx(ctx))
	response, err := c.Do(request)
	if err != nil {
		return nil, err
	}
	bytes, err := io.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return nil, err
	}
	switch response.StatusCode {
	default:
		var e struct {
			Error error `json:"error"`
		}

		if e := json.Unmarshal(bytes, &err); e != nil {
			return nil, errors.Errorf("status code: %d; %s",
				response.StatusCode, string(bytes))
		}
		return nil, errors.New(e.Error.Error())
	case http.StatusOK, http.StatusNoContent:
		return bytes, nil
	}
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
	if cacheDisabled, ok := envs["CACHE_DISABLED"]; ok {
		c.config.cacheDisabled, _ = strconv.ParseBool(cacheDisabled)
	}
	return nil
}

func (c *client) Open(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	switch c.config.protocol {
	default:
		return errors.Errorf("unsupported protocol: %s", c.config.protocol)
	case "http", "https":
		c.address = fmt.Sprintf("%s://%s", c.config.protocol,
			net.JoinHostPort(c.config.address, c.config.port))
	}
	if c.config.cacheDisabled {
		c.Info(ctx, "client: cache disabled")
	}
	c.Client.Timeout = time.Duration(c.config.timeout) * time.Second
	tlsConfig, err := getTlsConfig(c.config.sslCaFile, c.config.sslCrtFile,
		c.config.sslKeyFile)
	if err != nil {
		return err
	}
	c.Client.Transport = tlsConfig
	return nil
}

func (c *client) Close(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	return nil
}

func (c *client) EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	bytes, err := json.Marshal(&data.Request{
		EmployeePartial: employeePartial})
	if err != nil {
		return nil, err
	}
	uri := c.address + data.RouteEmployees
	bytes, err = c.doRequest(ctx, uri, http.MethodPut, bytes)
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
		c.Error(ctx, "error while reading employee (%d) from cache: %s\n", empNo, err)
	}
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	bytes, err := c.doRequest(ctx, uri, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesWrite(ctx, data.EmployeeSearch{}, response.Employee); err != nil {
			c.Error(ctx, "error while writing employee (%d) to cache: %s\n", empNo, err)
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
		c.Error(ctx, "error while reading employees from cache: %s\n", err)
	}
	params := search.ToParams()
	uri := c.address + data.RouteEmployeesSearch
	bytes, err := c.doRequest(ctx, uri, http.MethodGet, params)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesWrite(ctx, search, response.Employees...); err != nil {
			c.Error(ctx, "error while writing employees to cache: %s\n", err)
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
	bytes, err = c.doRequest(ctx, uri, http.MethodPost, bytes)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(ctx, empNo); err != nil {
			c.Error(ctx, "error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *client) EmployeeDelete(ctx context.Context, empNo int64) error {
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	if _, err := c.doRequest(ctx, uri, http.MethodDelete, nil); err != nil {
		return err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(ctx, empNo); err != nil {
			c.Error(ctx, "error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return nil
}

func (c *client) CacheClear(ctx context.Context) error {
	uri := fmt.Sprintf(c.address + data.RouteCache)
	if _, err := c.doRequest(ctx, uri, http.MethodDelete, nil); err != nil {
		return err
	}
	return nil
}

func (c *client) CacheCountersRead(ctx context.Context) (*data.CacheCounters, error) {
	uri := fmt.Sprintf(c.address + data.RouteCacheCounters)
	bytes, err := c.doRequest(ctx, uri, http.MethodGet, nil)
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
	if _, err := c.doRequest(ctx, uri, http.MethodDelete, nil); err != nil {
		return err
	}
	return nil
}

func (c *client) TimersRead(ctx context.Context) (*data.Timers, error) {
	uri := fmt.Sprintf(c.address + data.RouteTimers)
	bytes, err := c.doRequest(ctx, uri, http.MethodGet, nil)
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
	if _, err := c.doRequest(ctx, uri, http.MethodDelete, nil); err != nil {
		return err
	}
	return nil
}

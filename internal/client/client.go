package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-big-data/internal/cache"
	"github.com/antonio-alexander/go-blog-big-data/internal/data"
	"github.com/antonio-alexander/go-blog-big-data/internal/utilities"

	"github.com/pkg/errors"
)

type Client interface {
	Configure(envs map[string]string) error
	Open(correlationId string) error
	Close(correlationId string) error
	EmployeeCreate(correlationId string, ctx context.Context,
		employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeRead(correlationId string, ctx context.Context,
		empNo int64) (*data.Employee, error)
	EmployeesSearch(correlationId string, ctx context.Context,
		search data.EmployeeSearch) ([]*data.Employee, error)
	EmployeeUpdate(correlationId string, ctx context.Context,
		empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error)
	EmployeeDelete(correlationId string, ctx context.Context,
		empNo int64) error
}

type client struct {
	sync.RWMutex
	*http.Client
	utilities.Logger
	utilities.Timers
	cache  cache.Cache
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
}

func NewClient(parameters ...any) Client {
	c := &client{Client: &http.Client{}}
	for _, parameter := range parameters {
		switch p := parameter.(type) {
		case cache.Cache:
			c.cache = p
		case utilities.Logger:
			c.Logger = p
		case utilities.Timers:
			c.Timers = p
		}
	}
	return c
}

func (c *client) doRequest(correlationId string, ctx context.Context, uri, method string, item interface{}) ([]byte, error) {
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
	request.Header.Add("Correlation-Id", correlationId)
	request.Header.Add("Content-Type", contentType)
	request.Header.Add("Content-Length", strconv.Itoa(contentLength))
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

func (c *client) Error(correlationId, format string, v ...interface{}) {
	if c.Logger != nil {
		c.Logger.Error(correlationId, format, v...)
	}
}

func (c *client) Trace(correlationId, format string, v ...interface{}) {
	if c.Logger != nil {
		c.Logger.Trace(correlationId, format, v...)
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

func (c *client) Open(correlationId string) error {
	c.Lock()
	defer c.Unlock()

	c.address = strings.TrimPrefix(c.config.address, "https://")
	c.address = strings.TrimPrefix(c.config.address, "http://")
	if c.config.port != "" {
		c.address += ":" + c.config.port
	}
	switch c.config.protocol {
	default:
		return errors.Errorf("unsupported protocol: %s", c.config.protocol)
	case "http", "https":
		c.address = fmt.Sprintf("%s://%s", c.config.protocol, c.config.address)
		if c.config.port != "" {
			c.address += fmt.Sprintf(":%s", c.config.port)
		}
	}
	if c.config.cacheDisabled {
		c.Debug(correlationId, "cache disabled")
	}
	c.Client.Timeout = time.Duration(c.config.timeout) * time.Second
	tlsConfig, err := getTlsConfig(c.config.sslCaFile, c.config.sslCrtFile,
		c.config.sslKeyFile)
	if err != nil {
		return err
	}
	c.Client.Transport = tlsConfig
	c.Debug(correlationId, "client configured")
	return nil
}

func (c *client) Close(correlationId string) error {
	c.Lock()
	defer c.Unlock()

	return nil
}

func (c *client) EmployeeCreate(correlationId string, ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
	timerIndex := c.Start("employee_create")
	defer func() {
		elapsedtime := c.Stop("employee_create", timerIndex)
		c.Trace(correlationId, "employee_create took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	bytes, err := json.Marshal(&data.Request{
		EmployeePartial: employeePartial})
	if err != nil {
		return nil, err
	}
	uri := c.address + data.RouteEmployees
	bytes, err = c.doRequest(correlationId, ctx, uri, http.MethodPut, bytes)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	return response.Employee, nil
}

func (c *client) EmployeeRead(correlationId string, ctx context.Context, empNo int64) (*data.Employee, error) {
	timerIndex := c.Start("employee_read")
	defer func() {
		elapsedtime := c.Stop("employee_read", timerIndex)
		c.Trace(correlationId, "employee_read took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	if !c.config.cacheDisabled {
		employee, err := c.cache.EmployeeRead(correlationId, ctx, empNo)
		if err == nil {
			return employee, nil
		}
		c.Error(correlationId, "error while reading employee (%d) from cache: %s\n", empNo, err)
	}
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	bytes, err := c.doRequest(correlationId, ctx, uri, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesWrite(correlationId, ctx, data.EmployeeSearch{}, response.Employee); err != nil {
			c.Error(correlationId, "error while writing employee (%d) to cache: %s\n", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *client) EmployeesSearch(correlationId string, ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	var response data.Response

	timerIndex := c.Start("employees_search")
	defer func() {
		elapsedtime := c.Stop("employees_search", timerIndex)
		c.Trace(correlationId, "employees_search took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	if !c.config.cacheDisabled {
		employees, err := c.cache.EmployeesRead(correlationId, ctx, search)
		if err == nil {
			return employees, nil
		}
		c.Error(correlationId, "error while reading employees from cache: %s\n", err)
	}
	params := search.ToParams()
	uri := c.address + data.RouteEmployeesSearch
	bytes, err := c.doRequest(correlationId, ctx, uri, http.MethodGet, params)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesWrite(correlationId, ctx, search, response.Employees...); err != nil {
			c.Error(correlationId, "error while writing employees to cache: %s\n", err)
		}
	}
	return response.Employees, nil
}

func (c *client) EmployeeUpdate(correlationId string, ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
	timerIndex := c.Start("employee_update")
	defer func() {
		elapsedtime := c.Stop("employee_update", timerIndex)
		c.Trace(correlationId, "employee_update took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	bytes, err := json.Marshal(&data.Request{EmployeePartial: employeePartial})
	if err != nil {
		return nil, err
	}
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	bytes, err = c.doRequest(correlationId, ctx, uri, http.MethodPost, bytes)
	if err != nil {
		return nil, err
	}
	response := &data.Response{}
	if err := json.Unmarshal(bytes, response); err != nil {
		return nil, err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(correlationId, ctx, empNo); err != nil {
			c.Error(correlationId, "error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *client) EmployeeDelete(correlationId string, ctx context.Context, empNo int64) error {
	timerIndex := c.Start("employee_delete")
	defer func() {
		elapsedtime := c.Stop("employee_delete", timerIndex)
		c.Trace(correlationId, "employee_delete took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	if _, err := c.doRequest(correlationId, ctx, uri, http.MethodDelete, nil); err != nil {
		return err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(correlationId, ctx, empNo); err != nil {
			c.Error(correlationId, "error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return nil
}

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

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"

	"github.com/pkg/errors"
)

type Client struct {
	sync.RWMutex
	*http.Client
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

func NewClient(parameters ...interface{}) *Client {
	c := &Client{Client: &http.Client{}}
	for _, parameter := range parameters {
		switch p := parameter.(type) {
		case cache.Cache:
			c.cache = p
		}
	}
	return c
}

func (c *Client) doRequest(ctx context.Context, uri, method string, item interface{}) ([]byte, error) {
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

func (c *Client) Configure(envs map[string]string) error {
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

func (c *Client) Open() error {
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
		fmt.Println("cache disabled")
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

func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()

	return nil
}

func (c *Client) EmployeeCreate(ctx context.Context, employeePartial data.EmployeePartial) (*data.Employee, error) {
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

func (c *Client) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	if !c.config.cacheDisabled {
		employee, err := c.cache.EmployeeRead(ctx, empNo)
		if err == nil {
			return employee, nil
		}
		fmt.Printf("error while reading employee (%d) from cache: %s\n", empNo, err)
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
			fmt.Printf("error while writing employee (%d) to cache: %s\n", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *Client) EmployeesSearch(ctx context.Context, search data.EmployeeSearch) ([]*data.Employee, error) {
	var response data.Response

	if !c.config.cacheDisabled {
		employees, err := c.cache.EmployeesRead(ctx, search)
		if err == nil {
			return employees, nil
		}
		fmt.Printf("error while reading employees from cache: %s\n", err)
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
			fmt.Printf("error while writing employees to cache: %s\n", err)
		}
	}
	return response.Employees, nil
}

func (c *Client) EmployeeUpdate(ctx context.Context, empNo int64, employeePartial data.EmployeePartial) (*data.Employee, error) {
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
			fmt.Printf("error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return response.Employee, nil
}

func (c *Client) EmployeeDelete(ctx context.Context, empNo int64) error {
	uri := fmt.Sprintf(c.address+data.RouteEmployeesEmpNof, empNo)
	if _, err := c.doRequest(ctx, uri, http.MethodDelete, nil); err != nil {
		return err
	}
	if !c.config.cacheDisabled {
		if err := c.cache.EmployeesDelete(ctx, empNo); err != nil {
			fmt.Printf("error while deleting employee (%d) from cache: %s\n", empNo, err)
		}
	}
	return nil
}

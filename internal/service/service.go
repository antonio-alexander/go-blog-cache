package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"
	"github.com/antonio-alexander/go-blog-cache/internal/utilities"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
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

type Service struct {
	sync.RWMutex
	sync.WaitGroup
	context.Context
	*mux.Router
	*http.Server
	logic.Logic
	utilities.Logger
	utilities.Timers
	cancel context.CancelFunc
	config struct {
		address          string
		port             string
		timeout          time.Duration
		shutdownTimeout  time.Duration
		allowedOrigins   []string
		allowedMethods   []string
		allowedHeaders   []string
		allowCredentials bool
		corsDisabled     bool
		corsDebug        bool
	}
}

func NewService(parameters ...interface{}) *Service {
	router := mux.NewRouter()
	s := &Service{
		Router: router,
		Server: &http.Server{
			Handler: router,
		},
	}
	for _, parameter := range parameters {
		switch p := parameter.(type) {
		case logic.Logic:
			s.Logic = p
		case utilities.Logger:
			s.Logger = p
		case utilities.Timers:
			s.Timers = p
		}
	}
	return s
}

func (s *Service) launchServer(correlationId string) error {
	started := make(chan struct{})
	chErr := make(chan error, 1)
	s.Add(1)
	go func() {
		defer s.WaitGroup.Done()
		defer close(chErr)

		if !s.config.corsDisabled {
			s.Server.Handler = cors.New(cors.Options{
				AllowedOrigins:   s.config.allowedOrigins,
				AllowCredentials: s.config.allowCredentials,
				AllowedMethods:   s.config.allowedMethods,
				AllowedHeaders:   s.config.allowedHeaders,
				Debug:            s.config.corsDebug,
			}).Handler(s.Router)
		} else {
			s.Debug(correlationId, "CORS disabled")
		}
		close(started)
		if err := s.Server.ListenAndServe(); err != nil {
			chErr <- err
		}
	}()
	<-started
	select {
	case err := <-chErr:
		//KIM: here we're accounting for a situation where the server closes unexexpectedly
		// but quickly (within a second of starting); this allows us to respond to errors such as
		// the port being already used
		return err
	case <-time.After(time.Second):
		s.Debug(correlationId, "started server: %s:%s\n", s.config.address, s.config.port)
		return nil
	}
}

func (s *Service) endpointDefault() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer,
			"go-blog-cache\n"+
				"Version: \"%s\"\n"+
				"Git Commit: \"%s\"\n"+
				"Git Branch: \"%s\"\n",
			Version, GitCommit, GitBranch)
	}
}

func (s *Service) endpointEmployeeCreate(writer http.ResponseWriter, request *http.Request) {
	var employeeRequest data.Request

	ctx := request.Context()
	correlationId := getCorrelationId(request)
	timerIndex := s.Start("employee_create")
	defer func() {
		elapsedtime := s.Stop("employee_create", timerIndex)
		s.Trace(correlationId, "employee_create took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	bytes, err := io.ReadAll(request.Body)
	defer request.Body.Close()
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	if err := json.Unmarshal(bytes, &employeeRequest); err != nil {
		handleResponse(writer, err, nil)
		return
	}
	employee, err := s.EmployeeCreate(correlationId, ctx, employeeRequest.EmployeePartial)
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	handleResponse(writer, nil, &data.Response{
		Employee: employee,
	})
	s.Debug(correlationId, "created employee: %d\n", employee.EmpNo)
}

func (s *Service) endpointEmployeeRead(writer http.ResponseWriter, request *http.Request) {
	correlationId := getCorrelationId(request)
	timerIndex := s.Start("employee_read")
	defer func() {
		elapsedtime := s.Stop("employee_read", timerIndex)
		s.Trace(correlationId, "employee_read took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	ctx := request.Context()
	empNo, err := empNoFromPath(mux.Vars(request))
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	employee, err := s.EmployeeRead(correlationId, ctx, empNo)
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	handleResponse(writer, nil, &data.Response{
		Employee: employee,
	})
	s.Debug(correlationId, "read employee: %d\n", employee.EmpNo)
}

func (s *Service) endpointEmployeesSearch(writer http.ResponseWriter, request *http.Request) {
	var search data.EmployeeSearch

	correlationId := getCorrelationId(request)
	ctx := request.Context()
	timerIndex := s.Start("employees_search")
	defer func() {
		elapsedtime := s.Stop("employees_search", timerIndex)
		s.Trace(correlationId, "employees_search took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	if err := request.ParseForm(); err != nil {
		handleResponse(writer, err, nil)
		return
	}
	search.FromParams(request.Form)
	employees, err := s.EmployeesSearch(correlationId, ctx, search)
	if err != nil {
		handleResponse(writer, err)
		return
	}
	handleResponse(writer, nil, &data.Response{
		Employees: employees,
	})
	s.Debug(correlationId, "read employees\n")
}

func (s *Service) endpointEmployeeUpdate(writer http.ResponseWriter, request *http.Request) {
	var employeeRequest data.Request

	correlationId := getCorrelationId(request)
	ctx := request.Context()
	timerIndex := s.Start("employee_update")
	defer func() {
		elapsedtime := s.Stop("employee_update", timerIndex)
		s.Trace(correlationId, "employee_update took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	empNo, err := empNoFromPath(mux.Vars(request))
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	bytes, err := io.ReadAll(request.Body)
	defer request.Body.Close()
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	if err := json.Unmarshal(bytes, &employeeRequest); err != nil {
		handleResponse(writer, err, nil)
		return
	}
	employee, err := s.EmployeeUpdate(correlationId, ctx, empNo, employeeRequest.EmployeePartial)
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	handleResponse(writer, nil, &data.Response{
		Employee: employee,
	})
	s.Debug(correlationId, "updated employee: %d\n", employee.EmpNo)
}

func (s *Service) endpointEmployeeDelete(writer http.ResponseWriter, request *http.Request) {
	correlationId := getCorrelationId(request)
	ctx := request.Context()
	timerIndex := s.Start("employee_delete")
	defer func() {
		elapsedtime := s.Stop("employee_delete", timerIndex)
		s.Trace(correlationId, "employee_delete took %v",
			time.Duration(elapsedtime)*time.Nanosecond)
	}()
	empNo, err := empNoFromPath(mux.Vars(request))
	if err != nil {
		handleResponse(writer, err, nil)
		return
	}
	if err := s.EmployeeDelete(correlationId, ctx, empNo); err != nil {
		handleResponse(writer, err, nil)
		return
	}
	handleResponse(writer, nil, nil)
	s.Debug(correlationId, "deleted employee: %d\n", empNo)
}

func (s *Service) buildRoutes() {
	s.Router.HandleFunc("/", s.endpointDefault())
	s.Router.HandleFunc(data.RouteEmployeesSearch, s.endpointEmployeesSearch)
	s.Router.HandleFunc(data.RouteEmployees, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodPut:
			s.endpointEmployeeCreate(w, r)
		}
	})
	s.Router.HandleFunc(data.RouteEmployeesEmpNo, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodGet:
			s.endpointEmployeeRead(w, r)
		case http.MethodPost:
			s.endpointEmployeeUpdate(w, r)
		case http.MethodDelete:
			s.endpointEmployeeDelete(w, r)
		}
	})
}

func (s *Service) Configure(envs map[string]string) error {
	if address, ok := envs["SERVICE_ADDRESS"]; ok {
		s.config.address = address
	}
	if port, ok := envs["SERVICE_PORT"]; ok {
		s.config.port = port
	}
	if shutdownTimeoutString, ok := envs["SERVICE_SHUTDOWN_TIMEOUT"]; ok {
		if shutdownTimeoutInt, err := strconv.Atoi(shutdownTimeoutString); err == nil {
			if timeout := time.Duration(shutdownTimeoutInt) * time.Second; timeout > 0 {
				s.config.shutdownTimeout = timeout
			}
		}
	}
	if allowCredentialsString, ok := envs["SERVICE_CORS_ALLOW_CREDENTIALS"]; ok {
		if allowCredentials, err := strconv.ParseBool(allowCredentialsString); err == nil {
			s.config.allowCredentials = allowCredentials
		}
	}
	if allowedHeaders, ok := envs["SERVICE_CORS_ALLOWED_HEADERS"]; ok {
		s.config.allowedHeaders = strings.Split(allowedHeaders, ",")
	}
	if allowedOrigins, ok := envs["SERVICE_CORS_ALLOWED_ORIGINS"]; ok {
		s.config.allowedOrigins = strings.Split(allowedOrigins, ",")
	}
	if allowedMethods, ok := envs["SERVICE_CORS_ALLOWED_METHODS"]; ok {
		s.config.allowedMethods = strings.Split(allowedMethods, ",")
	}
	if corsDisabledString, ok := envs["SERVICE_CORS_DISABLED"]; ok {
		if corsDisabled, err := strconv.ParseBool(corsDisabledString); err == nil {
			s.config.corsDisabled = corsDisabled
		}
	}
	if corsDebug, ok := envs["SERVICE_CORS_DEBUG"]; ok {
		if corsDebug, err := strconv.ParseBool(corsDebug); err == nil {
			s.config.corsDebug = corsDebug
		}
	}
	return nil
}

func (s *Service) Open(correlationId string) error {
	s.Lock()
	defer s.Unlock()

	s.Context, s.cancel = context.WithCancel(context.Background())
	s.Server.Addr = fmt.Sprintf("%s:%s", s.config.address, s.config.port)
	s.buildRoutes()
	if err := s.launchServer(correlationId); err != nil {
		return err
	}
	return nil
}

func (s *Service) Close(correlationId string) error {
	s.Lock()
	defer s.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.shutdownTimeout)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		s.Error(correlationId, "error while shutting down the server: %s", err)
	}
	s.cancel()
	s.Wait()
	return nil
}

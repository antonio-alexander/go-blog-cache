package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/cache"
	"github.com/antonio-alexander/go-blog-cache/internal/data"
	"github.com/antonio-alexander/go-blog-cache/internal/logic"

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
	*logic.Logic
	cache  cache.Cache
	cancel context.CancelFunc
	config struct {
		address          string
		port             string
		timeout          time.Duration
		shutdownTimeout  time.Duration
		allowedOrigins   []string
		allowedMethods   []string
		allowCredentials bool
		corsDisabled     bool
		corsDebug        bool
		protocol         string
		sslCrtFile       string
		sslKeyFile       string
		cacheEnabled     bool
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
		case *logic.Logic:
			s.Logic = p
		case cache.Cache:
			s.cache = p
		}
	}
	return s
}

func (s *Service) launchServer() error {
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
				Debug:            s.config.corsDebug,
			}).Handler(s.Router)
		}
		close(started)
		switch s.config.protocol {
		default:
			if err := s.Server.ListenAndServe(); err != nil {
				chErr <- err
			}
		case "https":
			if err := s.Server.ListenAndServeTLS(s.config.sslCrtFile, s.config.sslKeyFile); err != nil {
				chErr <- err
			}
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
		fmt.Printf("started %s server: %s:%s\n", s.config.protocol, s.config.address, s.config.port)
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

func (s *Service) endpointEmployeeRead() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		empNo, err := empNoFromPath(mux.Vars(request))
		if err != nil {
			handleResponse(writer, err, nil)
			return
		}
		employee, err := s.EmployeeRead(ctx, empNo)
		if err != nil {
			handleResponse(writer, err, nil)
			return
		}
		if err := s.EmployeesWrite(s.Context, data.EmployeeSearch{}, employee); err != nil {
			fmt.Printf("error while writing employee (%d) to cache: %s\n",
				empNo, err)
		}
		handleResponse(writer, nil, employee)
	}
}

func (s *Service) endpointEmployeesSearch() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		var search data.EmployeeSearch

		ctx := request.Context()
		if err := request.ParseForm(); err != nil {
			handleResponse(writer, err, nil)
			return
		}
		search.FromParams(request.Form)
		employees, err := s.EmployeesSearch(ctx, search)
		if err != nil {
			handleResponse(writer, err)
			return
		}
		if err := s.EmployeesWrite(s.Context, search, employees...); err != nil {
			fmt.Printf("error while writing employees to cache: %s\n",
				err)
		}
		handleResponse(writer, nil, struct {
			Employees []*data.Employee `json:"employees"`
		}{
			Employees: employees,
		})
	}
}

func (s *Service) buildRoutes() {
	s.Router.HandleFunc("/", s.endpointDefault())
	s.Router.HandleFunc(data.RouteEmployeesSearch, s.endpointEmployeesSearch())
	s.Router.HandleFunc(data.RouteEmployeesNo, s.endpointEmployeeRead())
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
	if sslKeyFile, ok := envs["SSL_KEY_FILE"]; ok {
		s.config.sslKeyFile = sslKeyFile
	}
	if sslCrtFile, ok := envs["SSL_CRT_FILE"]; ok {
		s.config.sslCrtFile = sslCrtFile
	}
	if cacheEnabled, ok := envs["SERVICE_CACHE_ENABLED"]; ok {
		s.config.cacheEnabled, _ = strconv.ParseBool(cacheEnabled)
	}
	return nil
}

func (s *Service) Open() error {
	s.Lock()
	defer s.Unlock()

	s.Context, s.cancel = context.WithCancel(context.Background())
	s.Server.Addr = fmt.Sprintf("%s:%s", s.config.address, s.config.port)
	switch s.config.protocol {
	case "https":
		tlsCertificate, err := getCertificates(s.config.sslCrtFile, s.config.sslKeyFile)
		if err != nil {
			return err
		}
		s.Server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCertificate},
		}
	}
	s.buildRoutes()
	if err := s.launchServer(); err != nil {
		return err
	}
	if s.config.cacheEnabled {
		fmt.Println("cache enabled")
	}
	return nil
}

func (s *Service) Close() {
	s.Lock()
	defer s.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.shutdownTimeout)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		fmt.Printf("error while shutting down the server: %s\n", err)
	}
	s.cancel()
	s.Wait()
}

func (s *Service) EmployeesWrite(ctx context.Context, search data.EmployeeSearch, employees ...*data.Employee) error {
	if !s.config.cacheEnabled {
		return nil
	}
	return s.cache.EmployeesWrite(ctx, search, employees...)
}

func (s *Service) EmployeeRead(ctx context.Context, empNo int64) (*data.Employee, error) {
	if s.config.cacheEnabled {
		employee, err := s.cache.EmployeeRead(ctx, empNo)
		if err == nil {
			return employee, nil
		}
		fmt.Printf("error while reading employee (%d) from cache: %s\n", empNo, err)
	}
	return s.Sql.EmployeeRead(ctx, empNo)
}

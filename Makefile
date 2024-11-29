## ----------------------------------------------------------------------
## This Makefile contains multiple commands used for local dev ops 
## ----------------------------------------------------------------------

golint_version=v1.51.2
swagger_version=v0.29.0
swagger_port = 8082

env_file=.env # default env file
docker_args=-l error #default args, supresses warnings

mysql_root_password := `cat $(env_file) | grep MYSQL_ROOT_PASSWORD | sed 's/MYSQL_ROOT_PASSWORD=//g' | tr -d '"'`
mysql_user := `cat $(env_file) | grep MYSQL_USER | sed 's/MYSQL_USER=//g' | tr -d '"'`
mysql_password := `cat $(env_file) | grep MYSQL_PASSWORD | sed 's/MYSQL_PASSWORD=//g' | tr -d '"'`

.PHONY: help check-lint lint check-swagger swagger validate-swagger serve-swagger dep run build stop

# REFERENCE: https://stackoverflow.com/questions/16931770/makefile4-missing-separator-stop
help: ## - Show this help.
	@sed -ne '/@sed/!s/## //p' $(MAKEFILE_LIST)

check-lint: ## validate/install golangci-lint installation
	@which golangci-lint || (go install github.com/golangci/golangci-lint/cmd/golangci-lint@${golint_version})

lint: check-lint ## lint the source with verbose output
	@golangci-lint run --verbose

# Reference: https://medium.com/@pedram.esmaeeli/generate-swagger-specification-from-go-source-code-648615f7b9d9
check-swagger: ## - validate/install swagger (v0.29.0)
	@which swagger || (go install github.com/go-swagger/go-swagger/cmd/swagger@${swagger_version})

swagger: check-swagger ## - generate the swagger.json
	@swagger generate spec --work-dir=./internal/swagger --output ./tmp/swagger.json --scan-models

validate-swagger: swagger ## - validate the swagger.json
	@swagger validate ./tmp/swagger.json

serve-swagger: swagger ## - serve (web) the swagger.json
	@swagger serve -F=swagger ./tmp/swagger.json -p ${swagger_port} --no-open

build: ## build the test image
	@docker ${docker_args} compose --profile application build

dep: ## run all dependencies
	@docker ${docker_args} compose up --detach --wait

run: ## run all dependencies
	@docker ${docker_args} compose --profile application up --detach --wait

test: run
	@go test ./internal/... -cover -count=1 -v

stop: ## stop all dependencies and services
	@docker ${docker_args} compose --profile application down

clean: ## stop all dependencies and services and clear volumes
	@docker ${docker_args} compose --profile application down --volumes --remove-orphans

mysql-employees:
	@docker exec -it mysql sh /docker-entrypoint-initdb.d/001_load_employees.sh

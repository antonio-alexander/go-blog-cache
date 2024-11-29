## ----------------------------------------------------------------------
## This Makefile contains multiple commands used for local dev ops 
## ----------------------------------------------------------------------

golint_version=v1.51.2
swagger_version=v0.29.0
swagger_port = 8082
ssl_config_file=./config/openssl.conf

env_file=.env # default env file
docker_args=-l error #default args, supresses warnings
rsa_bits=4096 # number of bits to use for rsa key/cert generation

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

stop: ## stop all dependencies and services
	@docker ${docker_args} compose --profile application down

clean: ## stop all dependencies and services and clear volumes
	@docker ${docker_args} compose --profile application down --volumes --remove-orphans

check-openssl: ## Check if openssl is installed
	@which openssl || echo "openssl not found"

gen-https-certs: check-openssl ## Generate public/private SSL certificates
	@openssl req -x509 -newkey rsa:${rsa_bits} -sha256 -utf8 -days 1 -nodes \
	-config ${ssl_config_file} -keyout ./certs/ssl.key -out ./certs/ssl.crt 

mysql-employees:
	@docker exec -it mysql mysql -u ${mysql_user} -P ${mysql_password} < /tmp/test_db/employees.sql
test:
	ENV_FILE=/home/tyrfly1001/filestore-server/.env go test ./... -v  -failfast
test-coverage:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
run-mysql:
	docker-compose --env-file ./.env -f ./env/docker-compose.yml up -d
stop-mysql:
	docker-compose --env-file ./.env -f ./env/docker-compose.yml down
con-mysql:
	docker exec -it mysql-master mysql -uroot -pmaster_root_password filestore

.PHONY: test-coverage test run-mysql stop-mysql con-mysql con-mysql2

test:
	go test ./... -v 
test-coverage:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
run-mysql:
	cd /home/tyrfly1001/filestore-server/env
	docker-compose up -d
con-mysql:
	docker exec -it mysql-master mysql -uroot -pmaster_root_password filestore
con-mysql2:
	docker exec -it mysql-slave mysql -uroot -pslave_root_password filestore

.PHONY: test-coverage test run-mysql con-mysql con-mysql2
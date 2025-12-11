test:
	go test ./... -v 
test-coverage:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
mysql:
	cd /home/tyrfly1001/filestore-server/env
	docker-compose up -d
.PHONY: test-coverage test mysql
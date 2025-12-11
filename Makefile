test:
	go test ./... -v 
test-coverage:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
.PHONY: test-coverage test
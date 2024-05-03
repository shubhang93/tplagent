TEST_PKGS=$(shell go list ./... | grep -v actionable | grep -v sample | grep -v fatal | grep -v duration)

cover:
	go test -v -p 1 -count 1 -race -coverprofile cover.out $(TEST_PKGS)

test:
	go test -v -p 1 -count 1 -race $(TEST_PKGS)
compile:
	go build ./...
build:
	go build -o bin/tplagent ./cmd/...
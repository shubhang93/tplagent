TEST_PKGS=$(shell go list ./... | grep -v actionable | grep -v sample | grep -v fatal)

test:
	go test -v -p 1 -count 1 -race $(TEST_PKGS)
compile:
	go build ./...
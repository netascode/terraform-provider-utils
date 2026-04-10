default: test

# Run acceptance tests
.PHONY: test
test:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

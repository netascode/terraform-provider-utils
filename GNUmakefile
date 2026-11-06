default: test

# Run unit and acceptance tests
# Usage: make test [DEBUG=1] [TESTARGS="-run TestName"]
.PHONY: test
test:
	@$(if $(DEBUG),echo "Debug mode enabled - logs will be written to test-output.log";) \
	TF_ACC=1 \
	$(if $(DEBUG),TF_LOG=Trace) \
	go test -v $(TESTARGS) -count 1 -timeout 120m ./internal/provider $(if $(DEBUG),2>&1 | tee test-output.log)

# Format examples, regenerate changelog guide and documentation
.PHONY: gen
gen:
	go generate

# Build and install the provider
.PHONY: install
install:
	go install

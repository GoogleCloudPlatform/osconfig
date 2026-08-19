# OS Config E2E Tests (v2)

This package contains the test suites and infrastructure for running OS Config end-to-end (E2E) tests against Google Cloud Platform Compute Engine instances using standard Go testing tooling.

## Running Local Unit Tests

Local unit tests do not require GCP credentials or cloud resources:

```sh
cd e2e_tests_v2
go test ./...
```

## Running E2E Cloud Tests

1. Ensure you have Application Default Credentials:
   ```sh
   gcloud auth application-default login
   ```

2. Run the E2E tests with default settings:
   ```sh
   go test -tags=e2e -v -count=1 -parallel=5 -timeout=60m .
   ```

3. Override configuration using command-line flags:
   ```sh
   go test -tags=e2e -v -count=1 -parallel=5 -timeout=60m . \
     -project=my-test-project \
     -zone=us-central1-b \
     -max_concurrent_vms=10
   ```

4. Run a specific test case (use `-timeout=30m` for long-running tests like Windows):
   ```sh
   go test -tags=e2e -v -count=1 -timeout=30m -run '^TestOSInventory/windows-2016$' . \
     -project=my-test-project \
     -zone=us-central1-a
   ```

### Available Configuration Flags

| Flag | Default | Description |
|---|---|---|
| `-project` | `gcloud-parity-testing` | GCP project ID for test resources |
| `-zone` | `us-central1-a` | GCP zone for test instances |
| `-network` | `global/networks/default` | VPC network for test instances |
| `-subnetwork` | `""` | VPC subnetwork for test instances |
| `-service_account` | `default` | Service account for test instances |
| `-test_timeout` | `60m` | Execution timeout per test run |
| `-poll_interval` | `10s` | Polling interval for instance status and guest attributes |
| `-cleanup_timeout` | `5m` | Timeout for VM teardown and resource cleanup |
| `-max_concurrent_vms` | `5` | Maximum number of concurrent test instances |
| `-artifact_file` | `./artifacts/junit.xml` | Path to JUnit XML test report (and directory for test artifacts) |

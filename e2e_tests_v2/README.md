# OS Config E2E Tests (v2)

This package contains the test suites and infrastructure for running OS Config end-to-end (E2E) tests against Google Cloud Platform Compute Engine instances using standard Go testing tooling.

## Running Local Unit Tests

Local unit tests do not require GCP credentials or cloud resources:

```sh
cd e2e_tests_v2
go test ./...
```

## Running E2E Cloud Tests

1. Create a configuration file:
   ```sh
   cp config.example.json config.local.json
   ```
   Edit `config.local.json` with your test `project` and `zone`.

2. Ensure you have Application Default Credentials:
   ```sh
   gcloud auth application-default login
   ```

3. Run the E2E tests:
   ```sh
   E2E_CONFIG="$PWD/config.local.json" \
     go test -tags=e2e -v -count=1 -parallel=5 -timeout=60m .
   ```

4. Run a specific test case:
   ```sh
   E2E_CONFIG="$PWD/config.local.json" \
     go test -tags=e2e -v -count=1 -run 'TestOSInventory/debian-12' .
   ```

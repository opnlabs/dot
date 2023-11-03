# Done
[![Done](https://github.com/cvhariharan/done/actions/workflows/main.yml/badge.svg)](https://github.com/cvhariharan/done/actions/workflows/main.yml)

A minimal CI.

Runs the jobs inside docker containers.

## Building Done with Done
```bash
go run main.go -m
```
This should create an artifact tar file in the `.artifacts` directory with the linux binary for `done`.

`-m` flag gives access to the host's docker socket. This is required only if containers are created within `done`.
<p align="center">
    <img src="https://media.tenor.com/rKLBka9zl5UAAAAd/yeah-excellent.gif" width="40%" height="40%">
<p>

## Example
The job file is inspired from GitLab CI.

```yaml
stages:
  - build
  - test

jobs:
  - name: Build job
    stage: build
    image: "docker.io/alpine"
    variables:
      - TEST: testing123
    src: ./pkg
    script:
      - ls -al
      - echo "Building $TEST"
      - echo "Completed build"
    artifacts:
      - target/build

  - name: Validate
    stage: test
    image: "docker.io/alpine"
    script:
      - echo "Testing"
      - echo "Testing successful"
```


# Done
[![Done](https://github.com/cvhariharan/done/actions/workflows/main.yml/badge.svg)](https://github.com/cvhariharan/done/actions/workflows/main.yml)

A minimal CI. Designed to be local first.

Runs the jobs inside docker containers. Done communicates with the Docker daemon using the [Docker client API](https://pkg.go.dev/github.com/docker/docker/client#section-readme).

<p align="center">
    <img src="images/demo.gif" width="600">
<p>

## Features
- Single binary, can run anywhere, on your machine or CI/CD systems
- Uses plain Docker, no custom engines / runtimes
- Bring your own Docker images. Supports private registries
- Simple yaml job definition
- Multi stage builds with support for build artifacts

## Example
This project can be built with Done. The [done.yml](done.yml) file describes all the jobs necessary to build a linux binary.
```yaml
stages:
  - test
  - security
  - build

jobs:
  - name: Run tests
    stage: test
    image: "docker.io/golang:1.21.3"
    script:
      - go test ./...

  - name: Run checks
    stage: security
    image: "docker.io/golangci/golangci-lint:latest"
    script:
      - golangci-lint run ./...

  - name: Build job linux
    stage: build
    image: "docker.io/golang:1.21.3-bookworm"
    script:
      - git config --global safe.directory '*'
      - export VERSION=$(git describe --always)
      - export BUILDDATE=$(date)
      - export COMMIT=$(git log --format="%H" -n 1)
      - echo $BUILDDATE
      - |
        go build -o done \
        -ldflags="-X 'github.com/cvhariharan/done/cmd/done.version=$VERSION' \
        -X 'github.com/cvhariharan/done/cmd/done.builddate=$BUILDDATE' \
        -X 'github.com/cvhariharan/done/cmd/done.commit=$COMMIT'" \
        main.go
    artifacts:
      - done-ci
```

### Build Done with Done
Clone the repo and run

```bash
go run main.go -m
```
This should create an artifact tar file in the `.artifacts` directory with the linux binary for `done-ci`.

`-m` flag gives access to the host's docker socket. This is required only if containers are created within `done-ci`.
<p align="center">
    <img src="https://media.tenor.com/rKLBka9zl5UAAAAd/yeah-excellent.gif" width="40%" height="40%">
<p>


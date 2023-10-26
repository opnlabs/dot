# Done
A minimal CI.

Runs the jobs inside docker containers.

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
    script:
      - echo "Building project"
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
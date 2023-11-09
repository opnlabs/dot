FROM golang:1.21-alpine3.17 as Builder
ARG VERSION
ARG BUILDDATE
ARG COMMIT
WORKDIR /app
COPY . .
RUN go build -o dot -ldflags="-X 'github.com/opnlabs/dot/cmd/dot.version=$VERSION' \ 
    -X 'github.com/opnlabs/dot/cmd/dot.builddate=$BUILDDATE' \
    -X 'github.com/opnlabs/dot/cmd/dot.commit=$COMMIT'"

FROM alpine:3.18.2
COPY --from=Builder /app/dot /usr/bin/dot
WORKDIR /app
ENTRYPOINT ["/usr/bin/dot"]
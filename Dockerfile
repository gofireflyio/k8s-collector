# Builder stage
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.18.7-alpine3.16 as builder

ARG ACCESS_TOKEN_USR=""
ARG ACCESS_TOKEN_PWD=""

# git is required to fetch go dependencies
RUN printf "machine github.com\n\
    login ${ACCESS_TOKEN_USR}\n\
    password ${ACCESS_TOKEN_PWD}\n\
    \n\
    machine api.github.com\n\
    login ${ACCESS_TOKEN_USR}\n\
    password ${ACCESS_TOKEN_PWD}\n"\
    >> /root/.netrc

RUN chmod 600 /root/.netrc

RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

RUN CGO_ENABLED=0 go test ./...
RUN CGO_ENABLED=0 go build -o ifk8s main.go

# Final stage
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.16.1
RUN apk add --no-cache ca-certificates
COPY --from=builder /workspace/ifk8s /
COPY gitleaks.toml /
USER 65532:65532
ENTRYPOINT ["/ifk8s"]

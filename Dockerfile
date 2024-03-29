# Builder stage
FROM --platform=${TARGETPLATFORM:-linux/amd64} golangci/golangci-lint:v1.47.1-alpine AS builder
RUN apk --update add ca-certificates
WORKDIR /go/src/app
COPY . .
RUN go get -d ./...
RUN go test ./...
RUN CGO_ENABLED=0 go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o ifk8s main.go

# Final stage
FROM --platform=${TARGETPLATFORM:-linux/amd64} scratch
COPY --from=builder /go/src/app/ifk8s /
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
USER 65532:65532
ENTRYPOINT ["/ifk8s"]

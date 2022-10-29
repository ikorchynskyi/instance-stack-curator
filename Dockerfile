FROM golang:alpine AS builder

# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
WORKDIR "$GOPATH/src/instance-stack-curator"

# Fetch dependencies
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Build the binary
COPY . .
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64
RUN go build -ldflags="-w -s" -o "$GOPATH/bin/instance-stack-curator"

FROM scratch
COPY --from=builder /go/bin/instance-stack-curator /instance-stack-curator
ENTRYPOINT [ "/instance-stack-curator" ]

FROM golang:1.26-alpine AS build
WORKDIR /go/src/github.com/utilitywarehouse/ssh-key-manager
COPY . /go/src/github.com/utilitywarehouse/ssh-key-manager
# GOTOOLCHAIN pins the exact toolchain declared by go.mod's `go` line, so the
# build isn't at the mercy of whatever patch version the base image ships.
RUN apk --no-cache add git gcc musl-dev \
  && GOTOOLCHAIN=go$(awk '/^go /{print $2; exit}' go.mod) \
  && go mod download \
  && go test -v \
  && CGO_ENABLED=0 go build -o /ssh-key-manager .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
COPY --from=build /ssh-key-manager /ssh-key-manager
CMD [ "/ssh-key-manager" ]

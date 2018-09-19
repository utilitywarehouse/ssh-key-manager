FROM golang:alpine AS build
WORKDIR /go/src/app
COPY . /go/src/app/
RUN apk --no-cache add git gcc musl-dev &&\
 go get ./... &&\
 go test -v &&\
 CGO_ENABLED=0 go build -o /ssh-key-manager .

FROM alpine:3.8
RUN apk add --no-cache ca-certificates
COPY --from=build /ssh-key-manager /ssh-key-manager
CMD [ "/ssh-key-manager" ]

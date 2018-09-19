FROM alpine:3.8

ENV GOPATH=/go

WORKDIR /go/src/app
COPY . /go/src/app/

RUN apk --no-cache add ca-certificates go git musl-dev \
  && go get ./... \
  && go test -v \
  && CGO_ENABLED=0 go build -o /ssh-key-manager . \
  && apk del go git musl-dev \
  && rm -rf $GOPATH

CMD [ "/ssh-key-manager" ]

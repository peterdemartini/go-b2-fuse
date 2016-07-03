FROM golang:1.6
MAINTAINER Peter DeMartini <thepeterdemartini@gmail.com>

WORKDIR /go/src/github.com/peterdemartini/go-b2-fuse
COPY . /go/src/github.com/peterdemartini/go-b2-fuse

RUN env CGO_ENABLED=0 go build -o go-b2-fuse -a -ldflags '-s' .

CMD ["./go-b2-fuse"]

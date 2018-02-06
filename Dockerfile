FROM golang:1.9

WORKDIR /go/src/github.com/jeloou/rp/
COPY . .

RUN go get -u github.com/kardianos/govendor
RUN govendor install .

CMD ["rp"]

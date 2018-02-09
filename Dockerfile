FROM golang:1.10

ADD main.go .
RUN go build main.go
ENTRYPOINT [ "./main" ]
# build stage
FROM golang:1.19.0-alpine3.16

ADD . /go/src/github.com/cookielab/aws-instance-metadata-exporter
WORKDIR /go/src/github.com/cookielab/aws-instance-metadata-exporter
RUN go mod tidy
RUN go build -o /bin/aws-instance-metadata-exporter .

FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY --from=0 /bin/aws-instance-metadata-exporter /bin

USER nobody

ENTRYPOINT ["/bin/aws-instance-metadata-exporter"]

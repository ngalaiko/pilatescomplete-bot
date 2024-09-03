FROM golang:1.23.0-alpine3.20 as builder
WORKDIR /src
ADD go.mod go.sum
RUN go mod download
RUN go build -o /usr/bin/backend /src/cmd/server

FROM alpine:3.20
RUN apk add --no-cache curl 
COPY --from=builder /usr/bin/backend /usr/bin/backend
ENTRYPOINT [ "/usr/bin/backend", "--address", "0.0.0.0:8080", "--database-path", "/var/data" ]

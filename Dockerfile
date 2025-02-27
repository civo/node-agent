FROM golang:latest as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod tidy
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o node-agent .

FROM alpine:latest
COPY --from=builder /app/node-agent /usr/local/bin/node-agent
CMD ["node-agent"]

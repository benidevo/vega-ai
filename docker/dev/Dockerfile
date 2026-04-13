FROM golang:1.25-alpine3.22 AS dev

WORKDIR /app

RUN go install github.com/githubnemo/CompileDaemon@latest

COPY go.mod go.sum ./
RUN go mod download

EXPOSE 8765

CMD ["CompileDaemon", "--build=go build -o ./tmp/vega ./cmd/vega", "--command=./tmp/vega", "--color=true"]

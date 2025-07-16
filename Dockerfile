FROM golang:1.23
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server
EXPOSE 8080
CMD ["sh", "-c", "sleep 5 && ./server -config config.yaml -storage postgres"]
FROM golang:1.22-alpine as builder

WORKDIR /app

COPY . .

RUN go mod download
RUN go build -o main .
EXPOSE 3000
CMD ["./main"]

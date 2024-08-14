FROM golang:1.22-alpine
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main .
EXPOSE 3000
EXPOSE 6060
CMD ["./main"]

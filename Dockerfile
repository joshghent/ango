FROM golang:1.22-alpine

# Install required packages: make, migrate, and any dependencies
RUN apk add --no-cache make curl \
    && curl -L https://github.com/golang-migrate/migrate/releases/download/v4.15.2/migrate.linux-amd64.tar.gz | tar xvz -C /usr/local/bin \
    && chmod +x /usr/local/bin/migrate

WORKDIR /app
COPY . .

# Download Go modules
RUN go mod download

# Build the Go application
RUN go build -o main .

# Expose port 3000
EXPOSE 3000

# Command to run your application
CMD ["./main"]
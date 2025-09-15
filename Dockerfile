FROM golang:1.25.1-alpine

WORKDIR /app
COPY . .
RUN go build -o gophoto cmd/*.go
EXPOSE 8080
CMD ["./gophoto"]

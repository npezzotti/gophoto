FROM golang:alpine3.19
WORKDIR /app
COPY . .
RUN go build -o gophoto cmd/web/*.go
EXPOSE 8080
CMD ["./gophoto"]

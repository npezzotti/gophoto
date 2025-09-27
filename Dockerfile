FROM golang:1.25.1-alpine

# Install vips and its development files for bimg
RUN apk add --no-cache build-base vips-dev
WORKDIR /app
COPY . .
RUN go mod tidy
RUN make build
EXPOSE 8800
CMD ["./bin/gophoto"]

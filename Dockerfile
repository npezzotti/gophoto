FROM golang:1.26.5-alpine AS builder

RUN apk add --no-cache build-base vips-dev

WORKDIR /app

# Cache module downloads before copying the full source tree
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ./bin/gophoto ./cmd/*.go

FROM alpine AS runtime

# libvips runtime
RUN apk add --no-cache vips

WORKDIR /app
COPY --from=builder /app/bin/gophoto ./bin/gophoto

EXPOSE 8800
CMD ["./bin/gophoto"]

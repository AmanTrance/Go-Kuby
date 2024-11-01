FROM golang:1.23-alpine AS builder
WORKDIR /usr/src/server
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -o server .

FROM alpine:latest
COPY --from=builder /usr/src/server/server /
EXPOSE 8000
CMD ["/server"]





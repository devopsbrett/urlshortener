FROM golang:1.16.6-buster AS builder
WORKDIR /app
COPY . /app/
RUN go build .

FROM gcr.io/distroless/base:latest
WORKDIR /app/
COPY --from=builder /app/shortener ./
ENTRYPOINT ["./shortener"]
CMD ["-h"]
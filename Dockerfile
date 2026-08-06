FROM golang:alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /jobpin .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /jobpin /jobpin
ENTRYPOINT ["/jobpin"]

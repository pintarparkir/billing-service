FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/billing ./cmd/billing

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/billing /billing
EXPOSE 9091
ENTRYPOINT ["/billing"]

FROM golang:1.22.3-alpine AS builder

RUN apk update && apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o taskshed cmd/server/main.go

FROM alpine:3.20.0

WORKDIR /app

COPY --from=builder /app/taskshed /app/taskshed

COPY --from=builder /app/web /app/web

EXPOSE 8080

CMD ["/app/taskshed"]

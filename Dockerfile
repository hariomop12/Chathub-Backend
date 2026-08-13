FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN rm -f .env

RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata nginx nodejs npm
RUN npm install -g peer
RUN mkdir -p /run/nginx /app

WORKDIR /app

COPY --from=builder /app/server .
COPY nginx.conf /etc/nginx/http.d/default.conf

COPY start.sh /start.sh

EXPOSE 8080

CMD ["/start.sh"]

FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY configs ./configs
COPY images ./images
COPY internal ./internal
RUN go test ./... && go build -o /out/stonehenge-control ./cmd/stonehenge-control

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache curl docker-cli docker-cli-compose python3
COPY --from=build /out/stonehenge-control /app/stonehenge-control
COPY configs /app/configs
COPY migrations /app/migrations
COPY docker-compose.yml /app/docker-compose.yml
COPY tools /app/tools
ENV STONEHENGE_CONFIG=/app/configs/config.linux.example.json
EXPOSE 8080
ENTRYPOINT ["/app/stonehenge-control"]

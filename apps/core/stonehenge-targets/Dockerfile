FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /out/stonehenge-target-feed ./cmd/stonehenge-target-feed

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/stonehenge-target-feed /app/stonehenge-target-feed
USER 65532:65532
EXPOSE 8090
ENTRYPOINT ["/app/stonehenge-target-feed"]

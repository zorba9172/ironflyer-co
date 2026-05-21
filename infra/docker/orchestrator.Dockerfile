FROM golang:1.25-alpine AS build
WORKDIR /src
COPY apps/orchestrator/go.mod ./
RUN go mod download || true
COPY apps/orchestrator/ ./
RUN CGO_ENABLED=0 go build -o /out/orchestrator ./cmd/orchestrator

FROM alpine:3.20
RUN adduser -D -u 10001 iron
USER iron
COPY --from=build /out/orchestrator /usr/local/bin/orchestrator
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orchestrator"]

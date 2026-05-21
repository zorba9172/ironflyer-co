FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY apps/runtime/go.mod apps/runtime/go.sum ./
RUN go mod download
COPY apps/runtime/ ./
RUN CGO_ENABLED=0 go build -o /out/runtime ./cmd/runtime

# Final image includes a `git` binary so workspace git-clone works inside
# the container without requiring a fatter image like ubuntu.
FROM alpine:3.20
RUN apk add --no-cache git ca-certificates && adduser -D -u 10001 iron
USER iron
COPY --from=build /out/runtime /usr/local/bin/runtime
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/runtime"]

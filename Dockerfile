# Build stage: compile a static binary.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Download modules first so this layer is cached when only source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage: no shell or package manager, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080 50051
USER nonroot:nonroot
ENTRYPOINT ["/api"]

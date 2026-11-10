# syntax=docker/dockerfile:1

############################
# Stage 1: build
############################
FROM golang:1.27-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOWORK=off

# 1) dependencies first (cached until go.mod/go.sum change)
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./services/api-gateway/
RUN cd services/api-gateway && go mod download

# 2) source code
COPY pkg/ ./pkg/
COPY services/api-gateway/ ./services/api-gateway/

RUN cd services/api-gateway && \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd

############################
# Stage 2: runtime
############################
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]

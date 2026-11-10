# syntax=docker/dockerfile:1

############################
# Stage 1: build
############################
FROM golang:1.27-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOWORK=off

# 1) dependencies first (cached until go.mod/go.sum change)
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/todo-service/go.mod services/todo-service/go.sum ./services/todo-service/
RUN cd services/todo-service && go mod download

# 2) source code
COPY pkg/ ./pkg/
COPY services/todo-service/ ./services/todo-service/

RUN cd services/todo-service && \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd

############################
# Stage 2: runtime
############################
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/app /app
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app"]

FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gitcastle ./cmd/gitcastle

FROM alpine:3.21
RUN apk add --no-cache git ca-certificates
RUN addgroup -S gitcastle && adduser -S gitcastle -G gitcastle
WORKDIR /app
COPY --from=build /out/gitcastle /usr/local/bin/gitcastle
RUN mkdir -p /var/lib/gitcastle/repositories && chown -R gitcastle:gitcastle /var/lib/gitcastle
USER gitcastle
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENV REPOSITORY_ROOT=/var/lib/gitcastle/repositories
ENTRYPOINT ["gitcastle"]

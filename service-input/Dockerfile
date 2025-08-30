FROM golang:1.24 as build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o service-input

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /app/service-input .
EXPOSE 8080
ENTRYPOINT ["./service-input"]

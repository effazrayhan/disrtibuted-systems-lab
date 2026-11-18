FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/restaurant-service .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/restaurant-service /usr/local/bin/restaurant-service
USER 10001
EXPOSE 8081
ENTRYPOINT ["restaurant-service"]

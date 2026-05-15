# Build binaire Go (sans CGO → image finale légère)
FROM golang:1.22-alpine AS build
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rennes-emploi ./cmd/rennes-emploi

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl
WORKDIR /app
COPY --from=build /rennes-emploi /app/rennes-emploi
COPY public /app/public
ENV DATA_DIR=/app/data
ENV PORT=3000
RUN mkdir -p /app/data
EXPOSE 3000
CMD ["/app/rennes-emploi"]

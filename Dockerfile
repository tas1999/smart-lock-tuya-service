# syntax=docker/dockerfile:1
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/smartlock .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/smartlock /smartlock
USER nonroot:nonroot
EXPOSE 50051
ENTRYPOINT ["/smartlock"]

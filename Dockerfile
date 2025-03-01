FROM golang:alpine3.20 AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o /banhammer .

# This base image contains CA certificates we need for outbound HTTPS requests
# https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md
FROM gcr.io/distroless/static

COPY --from=build --chmod=755 "/banhammer" "/banhammer"

ENTRYPOINT [ "/banhammer" ]

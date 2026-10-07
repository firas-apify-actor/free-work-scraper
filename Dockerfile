FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /actor .

FROM gcr.io/distroless/static
COPY --from=build /actor /actor
ENTRYPOINT ["/actor"]

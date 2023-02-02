FROM golang:1.18 as build
WORKDIR /go/src/nats-testing
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/nats-testing

FROM gcr.io/distroless/static-debian11
COPY --from=build /go/bin/nats-testing /
CMD ["/nats-testing"]
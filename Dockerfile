FROM golang as builder
ENV GO111MODULE=on
WORKDIR /code
ADD . /code
RUN CGO_ENABLED=0 go build -o /weather .

FROM alpine:latest 
WORKDIR /
RUN apk add ca-certificates
COPY --from=builder /weather /weather
ENTRYPOINT ["/weather"]

FROM golang:alpine as builder
RUN mkdir -p /go/src/k8s-repetitious
ADD . /go/src/k8s-repetitious/
WORKDIR /go/src/k8s-repetitious
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates
RUN adduser -D -g '' appuser
RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .

FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/src/k8s-repetitious/main /app/
WORKDIR /app
USER appuser
ENTRYPOINT ["./main"]

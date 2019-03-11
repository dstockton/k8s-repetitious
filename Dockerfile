FROM golang:alpine as builder
RUN mkdir -p /go/src/github.com/dstockton/k8s-golang-react
ADD . /go/src/github.com/dstockton/k8s-golang-react/
WORKDIR /go/src/github.com/dstockton/k8s-golang-react
RUN apk add git
RUN go get
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .

FROM scratch
COPY --from=builder /go/src/github.com/dstockton/k8s-golang-react/main /app/
WORKDIR /app
CMD ["./main"]

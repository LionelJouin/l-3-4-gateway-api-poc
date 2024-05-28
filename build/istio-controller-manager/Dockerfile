ARG BUILD_VERSION=0.0.0-unknown

FROM golang:1.22 as build

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-extldflags -static -X github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/cli.Version=${BUILD_VERSION}" -o istio-controller-manager ./cmd/istio-controller-manager

FROM alpine

COPY --from=build /app/istio-controller-manager .

CMD ["./istio-controller-manager", "run"]

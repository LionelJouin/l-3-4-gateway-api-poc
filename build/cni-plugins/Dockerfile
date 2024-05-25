FROM golang:1.22 as build

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-extldflags -static" -o loopback-vip ./cmd/cni/loopback-vip \
  && CGO_ENABLED=0 GOOS=linux go build -ldflags "-extldflags -static" -o policy-route ./cmd/cni/policy-route

FROM golang:1.22 as cni-plugin-build

WORKDIR /app

ENV CNI_VERSION=v1.4.0

ADD https://api.github.com/repos/containernetworking/plugins/git/refs/tags/${CNI_VERSION} version.json

RUN git clone --depth 1 --branch ${CNI_VERSION} https://github.com/containernetworking/plugins.git ; \
    cd plugins ; \
    go mod download ; \
    CGO_ENABLED=0 ./build_linux.sh -ldflags "-extldflags -static -X github.com/containernetworking/plugins/pkg/utils/buildversion.BuildVersion=${CNI_VERSION}"

FROM alpine

WORKDIR /app

COPY --from=build /app/loopback-vip .
COPY --from=build /app/policy-route .
COPY --from=cni-plugin-build /app/plugins/bin/* .

CMD ["cp", "-a", "./", "/host/opt/cni/bin/"]
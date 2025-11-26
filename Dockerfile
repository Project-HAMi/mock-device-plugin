FROM golang:1.21-bullseye AS GOBUILD
ADD . /device-plugin
ARG GOPROXY=https://goproxy.cn,direct
RUN cd /device-plugin && go build -o ./k8s-device-plugin cmd/k8s-device-plugin/main.go

FROM ubuntu:22.04
WORKDIR /root/
COPY --from=GOBUILD /device-plugin/k8s-device-plugin .
CMD ["./k8s-device-plugin", "-logtostderr=true", "-stderrthreshold=INFO", "-v=5"]

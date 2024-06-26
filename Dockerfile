# Copyright 2018 Advanced Micro Devices, Inc.  All rights reserved.
# 
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
# 
#      http://www.apache.org/licenses/LICENSE-2.0
# 
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
FROM golang:1.21-bullseye AS GOBUILD
ADD . /device-plugin
ARG GOPROXY=https://goproxy.cn,direct
RUN cd /device-plugin && go build -o ./k8s-device-plugin cmd/k8s-device-plugin/main.go

FROM ubuntu:20.04
WORKDIR /root/
COPY --from=GOBUILD /device-plugin/k8s-device-plugin .
CMD ["./k8s-device-plugin", "-logtostderr=true", "-stderrthreshold=INFO", "-v=5"]

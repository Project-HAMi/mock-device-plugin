/*
Copyright 2024 The HAMi Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package device

import (
	"context"
	"flag"
	"os"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/ascend"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/cambricon"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/iluvatar"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/nvidia"
	"github.com/HAMi/mock-device-plugin/internal/pkg/util/client"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type Devices interface {
	RunManager()
}

var (
	HandshakeAnnos  = map[string]string{}
	RegisterAnnos   = map[string]string{}
	DevicesToHandle []string
	ch              = map[string]chan int{}
)

var devices map[string]Devices
var DebugMode bool

func GetDevices() map[string]Devices {
	return devices
}

func Initialize() {
	devices = make(map[string]Devices)
	nodeName := os.Getenv("NODE_NAME")
	node, err := client.GetClient().CoreV1().Nodes().Get(context.Background(), nodeName, v1.GetOptions{})
	if err != nil {
		klog.Infoln("Get node error", err.Error())
	}
	nvidiadevice := nvidia.InitNvidiaDevice(node)
	if nvidiadevice != nil {
		devices["NVIDIA"] = nvidiadevice
		ch["NVIDIA"] = make(chan int)
	}
	cambricondevice := cambricon.InitCambriconDevice(node)
	if cambricondevice != nil {
		devices["CAMBRICON"] = cambricondevice
		ch["CAMBRICON"] = make(chan int)
	}
	iluvatardevice := iluvatar.InitIluvatarGPUDevice(node)
	if iluvatardevice != nil {
		devices["Iluvatar"] = iluvatardevice
		ch["Iluvatar"] = make(chan int)
	}
	ascenddevice := ascend.InitAscendDevice(node)
	if ascenddevice != nil {
		devices["Ascend"] = ascenddevice
		ch["Ascend"] = make(chan int)
	}
}

func GlobalFlagSet() {
	nvidia.ParseConfig()
	cambricon.ParseConfig()
	flag.BoolVar(&DebugMode, "debug", false, "debug mode")
}

func RunManagers() {
	for idx, val := range devices {
		klog.Infoln("val.Name=", idx)
		go val.RunManager()
	}
	for _, val := range ch {
		<-val
	}
}

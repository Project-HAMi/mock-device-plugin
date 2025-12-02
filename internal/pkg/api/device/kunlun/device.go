/*
Copyright 2025 The HAMi Authors.

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

package kunlun

import (
	"flag"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	XPUDevice      = "XPU"
	XPUCommonWord  = "XPU"
	RegisterAnnos  = "hami.io/node-register-xpu"
	HandshakeAnnos = "hami.io/node-handshake-xpu"
)

type KunlunConfig struct {
	ResourceCountName   string `yaml:"resourceCountName"`
	ResourceVCountName  string `yaml:"resourceVCountName"`
	ResourceVMemoryName string `yaml:"resourceVMemoryName"`
}

type KunlunVDevices struct {
}

func InitKunlunVDevice(config KunlunConfig) *KunlunVDevices {
	KunlunResourceVCount = config.ResourceVCountName
	KunlunResourceVMemory = config.ResourceVMemoryName
	return &KunlunVDevices{}
}

func (dev *KunlunVDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	anno, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, fmt.Errorf("annos not found %s", RegisterAnnos)
	}
	nodeDevices, err := device.UnMarshalNodeDevices(anno)
	if err != nil {
		klog.ErrorS(err, "failed to unmarshal node devices", "node", n.Name, "device annotation", anno)
		return []*device.DeviceInfo{}, err
	}
	for idx := range nodeDevices {
		nodeDevices[idx].DeviceVendor = dev.CommonWord()
	}
	if len(nodeDevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", anno)
		return []*device.DeviceInfo{}, errors.New("no device found on node")
	}
	return nodeDevices, nil
}
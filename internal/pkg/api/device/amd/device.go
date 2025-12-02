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

package amd

import (
	"fmt"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

const (
	AMDDevice          = "AMDGPU"
	AMDCommonWord      = "AMDGPU"
	AMDDeviceSelection = "amd.com/gpu-index"
	AMDUseUUID         = "amd.com/use-gpu-uuid"
	AMDNoUseUUID       = "amd.com/nouse-gpu-uuid"
	AMDAssignedNode    = "amd.com/predicate-node"
	Mi300xMemory       = 192000
)

type AMDConfig struct {
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
}

type AMDDevices struct {
	resourceCountName  string
	resourceMemoryName string
}

func ParseConfig() {
}

func InitAMDGPUDevice(config AMDConfig) *AMDDevices {
	return &AMDDevices{
		resourceCountName:  config.ResourceCountName,
		resourceMemoryName: config.ResourceMemoryName,
	}
}

func (dev *AMDDevices) CommonWord() string {
	return AMDCommonWord
}

func (dev *AMDDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	nodedevices := []*device.DeviceInfo{}
	i := 0
	counts, ok := n.Status.Capacity.Name(corev1.ResourceName(dev.resourceCountName), resource.DecimalSI).AsInt64()
	if !ok || counts == 0 {
		return []*device.DeviceInfo{}, fmt.Errorf("device not found %s", dev.resourceCountName)
	}
	for int64(i) < counts {
		nodedevices = append(nodedevices, &device.DeviceInfo{
			Index:        uint(i),
			ID:           n.Name + "-" + AMDDevice + "-" + fmt.Sprint(i),
			Count:        1,
			Devmem:       Mi300xMemory,
			Devcore:      100,
			Type:         AMDDevice,
			Numa:         0,
			Health:       true,
			CustomInfo:   make(map[string]any),
			DeviceVendor: AMDCommonWord,
		})
		i++
	}
	i = 0
	for i < len(nodedevices) {
		klog.V(4).Infoln("Registered AMD nodedevices:", nodedevices[i])
		i++
	}
	return nodedevices, nil
}
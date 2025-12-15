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

package enflame

import (
	"fmt"

	//"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"

	//"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	// "k8s.io/klog/v2"
)

type EnflameDevices struct {
	factor int
}

type EnflameConfig struct {
	// GCU
	ResourceNameGCU string `yaml:"resourceNameGCU"`

	// Shared-GCU
	ResourceNameVGCU           string `yaml:"resourceNameVGCU"`
	ResourceNameVGCUPercentage string `yaml:"resourceNameVGCUPercentage"`
}

const (
	EnflameVGCUDevice     = "Enflame"
	EnflameVGCUCommonWord = "Enflame"
	// IluvatarUseUUID is user can use specify Iluvatar device for set Iluvatar UUID.
	EnflameUseUUID = "enflame.com/use-gpuuuid"
	// IluvatarNoUseUUID is user can not use specify Iluvatar device for set Iluvatar UUID.
	EnflameNoUseUUID   = "enflame.com/nouse-gpuuuid"
	PodRequestGCUSize  = "enflame.com/gcu-request-size"
	PodAssignedGCUID   = "enflame.com/gcu-assigned-id"
	PodHasAssignedGCU  = "enflame.com/gcu-assigned"
	PodAssignedGCUTime = "enflame.com/gcu-assigned-time"
	GCUSharedCapacity  = "enflame.com/gcu-shared-capacity"

	SharedResourceName = "enflame.com/shared-gcu"
	CountNoSharedName  = "enflame.com/gcu-count"
)

var (
	EnflameResourceNameVGCU           string
	EnflameResourceNameVGCUPercentage string
)

func InitEnflameDevice(config EnflameConfig) *EnflameDevices {
	EnflameResourceNameVGCU = config.ResourceNameVGCU
	EnflameResourceNameVGCUPercentage = config.ResourceNameVGCUPercentage
	return &EnflameDevices{
		factor: 0,
	}
}

func (dev *EnflameDevices) CommonWord() string {
	return EnflameVGCUCommonWord
}

func (dev *EnflameDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	nodedevices := []*device.DeviceInfo{}
	i := 0
	cards, ok := n.Status.Capacity.Name(corev1.ResourceName(CountNoSharedName), resource.DecimalSI).AsInt64()
	if !ok || cards == 0 {
		return []*device.DeviceInfo{}, fmt.Errorf("device not found %s", CountNoSharedName)
	}
	shared, _ := n.Status.Capacity.Name(corev1.ResourceName(SharedResourceName), resource.DecimalSI).AsInt64()
	dev.factor = int(shared / cards)
	for i < int(cards) {
		nodedevices = append(nodedevices, &device.DeviceInfo{
			Index:        uint(i),
			ID:           n.Name + "-enflame-" + fmt.Sprint(i),
			Count:        100,
			Devmem:       100,
			Devcore:      100,
			Type:         EnflameVGCUDevice,
			Numa:         0,
			Health:       true,
			DeviceVendor: EnflameVGCUCommonWord,
		})
		i++
	}
	return nodedevices, nil
}

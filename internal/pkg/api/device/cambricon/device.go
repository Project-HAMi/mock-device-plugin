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

package cambricon

import (
	"fmt"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

type CambriconConfig struct {
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
	ResourceCoreName   string `yaml:"resourceCoreName"`
}

const (
	CambriconMLUDevice     = "MLU"
	CambriconMLUCommonWord = "MLU"
)

var (
	MLUResourceCount  string
	MLUResourceMemory string
	MLUResourceCores  string
)

var (
	ResourceName string
)

type CambriconDevices struct {
}

func InitMLUDevice(config CambriconConfig) *CambriconDevices {
	MLUResourceCount = config.ResourceCountName
	MLUResourceMemory = config.ResourceMemoryName
	MLUResourceCores = config.ResourceCoreName
	return &CambriconDevices{}
}

func (dev *CambriconDevices) CommonWord() string {
	return CambriconMLUCommonWord
}

func (dev *CambriconDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	nodedevices := []*device.DeviceInfo{}
	i := 0
	cards, ok := n.Status.Capacity.Name(corev1.ResourceName(MLUResourceCores), resource.DecimalSI).AsInt64()
	if !ok || cards == 0 {
		return []*device.DeviceInfo{}, fmt.Errorf("device not found %s", MLUResourceCores)
	}
	memoryTotal, _ := n.Status.Capacity.Name(corev1.ResourceName(MLUResourceMemory), resource.DecimalSI).AsInt64()
	for int64(i)*100 < cards {
		nodedevices = append(nodedevices, &device.DeviceInfo{
			Index:        uint(i),
			ID:           n.Name + "-cambricon-mlu-" + fmt.Sprint(i),
			Count:        100,
			Devmem:       int32(memoryTotal * 256 * 100 / cards),
			Devcore:      100,
			Type:         CambriconMLUDevice,
			Numa:         0,
			Health:       true,
			DeviceVendor: CambriconMLUCommonWord,
		})
		i++
	}
	return nodedevices, nil
}

type CambriconMLUDevices struct {
	DM *dpm.Manager
}

func InitCambriconDevice(n *corev1.Node) *CambriconMLUDevices {
	num, ok := n.Status.Allocatable["cambricon.com/real-mlu-counts"]
	if !ok {
		return nil
	}
	count, ok := num.AsInt64()
	if !ok {
		return nil
	}

	dev := &CambriconMLUDevices{}
	index := strings.Index(ResourceName, "/")
	mock.Counts[ResourceName[index+1:]] = int(count) * 10
	return dev
}

func (dev *CambriconMLUDevices) RunManager() {
	klog.Infoln("runManager.....")
	index := strings.Index(ResourceName, "/")
	lmock := mock.MockLister{
		ResUpdateChan: make(chan dpm.PluginNameList),
		Heartbeat:     make(chan bool),
		Namespace:     ResourceName[:index],
	}
	mockmanager := dpm.NewManager(&lmock)

	go func() {
		lmock.ResUpdateChan <- []string{ResourceName[index+1:]}
	}()
	klog.Infoln("Running mocking dp:cambricon")
	mockmanager.Run()
}
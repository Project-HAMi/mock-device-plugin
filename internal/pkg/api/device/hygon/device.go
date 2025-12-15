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

package hygon

import (
	"errors"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type HygonConfig struct {
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
	ResourceCoreName   string `yaml:"resourceCoreName"`
	MemoryFactor       int32  `yaml:"memoryFactor"`
}

type DCUDevices struct {
	resourceNames []string
}

var (
	HygonResourceCount  string
	HygonResourceMemory string
	HygonResourceCores  string
	MemoryFactor        int32
)

const (
	RegisterAnnos      = "hami.io/node-dcu-register"
	HygonDCUDevice     = "DCU"
	HygonDCUCommonWord = "DCU"
)

func InitDCUDevice(config HygonConfig) *DCUDevices {
	HygonResourceCount = config.ResourceCountName
	HygonResourceMemory = config.ResourceMemoryName
	HygonResourceCores = config.ResourceCoreName
	MemoryFactor = config.MemoryFactor
	return &DCUDevices{}
}

func (dev *DCUDevices) CommonWord() string {
	return HygonDCUCommonWord
}

func (dev *DCUDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodedevices, err := device.DecodeNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, err
	}
	for idx := range nodedevices {
		nodedevices[idx].DeviceVendor = HygonDCUCommonWord
	}
	if len(nodedevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, errors.New("no gpu found on node")
	}
	devDecoded := device.EncodeNodeDevices(nodedevices)
	klog.V(5).InfoS("nodes device information", "node", n.Name, "nodedevices", devDecoded)
	return nodedevices, nil
}

func (dev *DCUDevices) AddResource(n corev1.Node) {
	devs, err := dev.GetNodeDevices(n)
	if err != nil {
		klog.Infof("no device %s on this node", dev.CommonWord())
		return
	}
	memoryResourceName := device.GetResourceName(HygonResourceMemory)
	for _, val := range devs {
		mock.Counts[memoryResourceName] += int(val.Devmem)
	}
	if MemoryFactor > 1 {
		rawMemory := mock.Counts[memoryResourceName]
		mock.Counts[memoryResourceName] /= int(MemoryFactor)
		klog.InfoS("Update memory", "raw", rawMemory, "after", mock.Counts[memoryResourceName], "factor", MemoryFactor)
	}
	klog.InfoS("Add resources", memoryResourceName, mock.Counts[memoryResourceName])
	dev.resourceNames = append(dev.resourceNames, memoryResourceName)
}

func (dev *DCUDevices) RunManager() {
	lmock := mock.MockLister{
		ResUpdateChan: make(chan dpm.PluginNameList),
		Heartbeat:     make(chan bool),
		Namespace:     device.GetVendorName(HygonResourceMemory),
	}
	go func() {
		lmock.ResUpdateChan <- dev.resourceNames
	}()
	mockmanager := dpm.NewManager(&lmock)
	klog.Infoln("Running mocking dp: nvidia")
	mockmanager.Run()
}

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

package ascend

import (
	"errors"
	"fmt"
	"sort"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type Template struct {
	Name   string `yaml:"name"`
	Memory int64  `yaml:"memory"`
	AICore int32  `yaml:"aiCore,omitempty"`
	AICPU  int32  `yaml:"aiCPU,omitempty"`
}

type VNPUConfig struct {
	CommonWord         string     `yaml:"commonWord"`
	ChipName           string     `yaml:"chipName"`
	ResourceName       string     `yaml:"resourceName"`
	ResourceMemoryName string     `yaml:"resourceMemoryName"`
	MemoryAllocatable  int64      `yaml:"memoryAllocatable"`
	MemoryCapacity     int64      `yaml:"memoryCapacity"`
	MemoryFactor       int32      `yaml:"memoryFactor"`
	AICore             int32      `yaml:"aiCore"`
	AICPU              int32      `yaml:"aiCPU"`
	Templates          []Template `yaml:"templates"`
}

type Devices struct {
	config           VNPUConfig
	nodeRegisterAnno string
}

func InitDevices(config []VNPUConfig) []*Devices {
	var devs []*Devices
	for _, vnpu := range config {
		commonWord := vnpu.CommonWord
		dev := &Devices{
			config:           vnpu,
			nodeRegisterAnno: fmt.Sprintf("hami.io/node-register-%s", commonWord),
		}
		sort.Slice(dev.config.Templates, func(i, j int) bool {
			return dev.config.Templates[i].Memory < dev.config.Templates[j].Memory
		})
		devs = append(devs, dev)
		klog.Infof("load ascend vnpu config %s: %v", commonWord, dev.config)
	}
	return devs
}

func (dev *Devices) CommonWord() string {
	return dev.config.CommonWord
}

func (dev *Devices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	anno, ok := n.Annotations[dev.nodeRegisterAnno]
	if !ok {
		return []*device.DeviceInfo{}, fmt.Errorf("annos not found %s", dev.nodeRegisterAnno)
	}
	nodeDevices, err := device.UnMarshalNodeDevices(anno)
	for idx := range nodeDevices {
		nodeDevices[idx].DeviceVendor = dev.config.CommonWord
	}
	if err != nil {
		klog.ErrorS(err, "failed to unmarshal node devices", "node", n.Name, "device annotation", anno)
		return []*device.DeviceInfo{}, err
	}
	if len(nodeDevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", anno)
		return []*device.DeviceInfo{}, errors.New("no device found on node")
	}
	return nodeDevices, nil
}

func (dev *Devices) GetResource(n corev1.Node) map[string]int {
	resourceName := device.GetResourceName(dev.config.ResourceMemoryName)
	resourceMap := map[string]int{
		resourceName: 0,
	}
	devInfos, err := dev.GetNodeDevices(n)
	if err != nil || len(devInfos) == 0 {
		klog.Infof("no device %s on this node", dev.config.CommonWord)
		return resourceMap
	}
	for _, val := range devInfos {
		resourceMap[resourceName] += int(val.Devmem)
	}
	if dev.config.MemoryFactor > 1 {
		rawMemory := resourceMap[resourceName]
		resourceMap[resourceName] /= int(dev.config.MemoryFactor)
		klog.InfoS("Update memory", "raw", rawMemory, "after", resourceMap[resourceName], "factor", dev.config.MemoryFactor)
	}
	klog.InfoS("Add resource", resourceName, resourceMap[resourceName])
	return resourceMap
}

func (dev *Devices) RunManager(n corev1.Node) {
	lmock := mock.NewMockLister(device.GetVendorName(dev.config.ResourceMemoryName))
	go device.Register(n, lmock, dev)
	mockmanager := dpm.NewManager(lmock)
	klog.Infof("Running mocking dp: %s", dev.CommonWord())
	mockmanager.Run()
}

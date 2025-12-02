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
	"flag"
	"strings"

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
	AICore             int32      `yaml:"aiCore"`
	AICPU              int32      `yaml:"aiCPU"`
	Templates          []Template `yaml:"templates"`
}

type Devices struct {
	config           VNPUConfig
	nodeRegisterAnno string
	useUUIDAnno      string
	noUseUUIDAnno    string
	handshakeAnno    string
}

func InitDevices(config []VNPUConfig) []*Devices {
	var devs []*Devices
	if !enableAscend {
		return devs
	}
	for _, vnpu := range config {
		commonWord := vnpu.CommonWord
		dev := &Devices{
			config:           vnpu,
			nodeRegisterAnno: fmt.Sprintf("hami.io/node-register-%s", commonWord),
			useUUIDAnno:      fmt.Sprintf("hami.io/use-%s-uuid", commonWord),
			noUseUUIDAnno:    fmt.Sprintf("hami.io/no-use-%s-uuid", commonWord),
			handshakeAnno:    fmt.Sprintf("hami.io/node-handshake-%s", commonWord),
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

var (
	ResourceMemoryName string
)

type AscendNPUDevices struct {
	DM *dpm.Manager
}

func InitAscendDevice(n *v1.Node) *AscendNPUDevices {
	num, ok := n.Status.Allocatable["huawei.com/Ascend910"]
	if !ok {
		return nil
	}
	count, ok := num.AsInt64()
	if !ok {
		return nil
	}

	dev := &AscendNPUDevices{}
	index := strings.Index(ResourceMemoryName, "/")
	mock.Counts[ResourceMemoryName[index+1:]] = int(count) / 10 * 65536
	return dev
}

func (dev *AscendNPUDevices) RunManager() {
	klog.Infoln("runManager.....")
	index := strings.Index(ResourceMemoryName, "/")
	lmock := mock.MockLister{
		ResUpdateChan: make(chan dpm.PluginNameList),
		Heartbeat:     make(chan bool),
		Namespace:     ResourceMemoryName[:index],
	}
	mockmanager := dpm.NewManager(&lmock)

	go func() {
		lmock.ResUpdateChan <- []string{ResourceMemoryName[index+1:]}
	}()
	klog.Infoln("Running mocking dp:ascend")
	mockmanager.Run()
}

func ParseConfig() {
	flag.StringVar(&ResourceMemoryName, "mlu-resource-memory-name", "huawei.com/Ascend910-memory", "virtual memory for npu to be allocated")
}

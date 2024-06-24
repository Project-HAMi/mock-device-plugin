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

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

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

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

package iluvatar

import (
	"flag"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

var (
	ResourceName     string
	ResourceCoreName string
)

type IluvatarGPUDevices struct {
	DM *dpm.Manager
}

func InitIluvatarGPUDevice(n *v1.Node) *IluvatarGPUDevices {
	num, ok := n.Status.Allocatable[v1.ResourceName(ResourceCoreName)]
	if !ok {
		return nil
	}
	count, ok := num.AsInt64()
	if !ok {
		return nil
	}

	dev := &IluvatarGPUDevices{}
	index := strings.Index(ResourceName, "/")
	mock.Counts[ResourceName[index+1:]] = int(count) / 10
	return dev
}

func (dev *IluvatarGPUDevices) RunManager() {
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
	klog.Infoln("Running mocking dp:iluvatar")
	mockmanager.Run()
}

func ParseConfig() {
	flag.StringVar(&ResourceName, "iluvatar-resource-name", "iluvatar.ai/vgpu", "virtual devices for iluvatar to be allocated")
	flag.StringVar(&ResourceCoreName, "iluvatar-core-name", "iluvatar.ai/MR-V100.vCore", "virtual cores for iluvatar to be allocated")
}

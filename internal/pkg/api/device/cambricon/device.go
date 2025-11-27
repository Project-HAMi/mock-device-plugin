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
	"flag"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type CambriconConfig struct {
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
	ResourceCoreName   string `yaml:"resourceCoreName"`
}

var (
	ResourceName string
)

type CambriconMLUDevices struct {
	DM *dpm.Manager
}

func InitCambriconDevice(n *v1.Node) *CambriconMLUDevices {
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

func ParseConfig() {
	flag.StringVar(&ResourceName, "mlu-resource-name", "cambricon.com/vmlu", "virtual devices for mlu to be allocated")
}

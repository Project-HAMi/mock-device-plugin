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
	"errors"
	"fmt"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	//"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	XPUDevice      = "XPU"
	XPUCommonWord  = "XPU"
	RegisterAnnos  = "hami.io/node-register-xpu"
	HandshakeAnnos = "hami.io/node-handshake-xpu"
)

var (
	KunlunResourceVCount  string
	KunlunResourceVMemory string
)

type KunlunConfig struct {
	ResourceCountName   string `yaml:"resourceCountName"`
	ResourceVCountName  string `yaml:"resourceVCountName"`
	ResourceVMemoryName string `yaml:"resourceVMemoryName"`
}

type KunlunVDevices struct {
	resourceNames []string
}

func InitKunlunVDevice(config KunlunConfig) *KunlunVDevices {
	KunlunResourceVCount = config.ResourceVCountName
	KunlunResourceVMemory = config.ResourceVMemoryName
	return &KunlunVDevices{}
}

func (dev *KunlunVDevices) CommonWord() string {
	return XPUDevice
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

func (dev *KunlunVDevices) AddResource(n corev1.Node) {
	devInfos, err := dev.GetNodeDevices(n)
	if err != nil || len(devInfos) == 0 {
		klog.Infof("no device %s on this node", dev.CommonWord())
		return
	}
	memoryResourceName := device.GetResourceName(KunlunResourceVMemory)
	vCountResourceName := device.GetResourceName(KunlunResourceVCount)
	for _, val := range devInfos {
		mock.Counts[vCountResourceName] += int(val.Devcore)
		mock.Counts[memoryResourceName] += int(val.Devmem)
	}
	dev.resourceNames = append(dev.resourceNames, vCountResourceName, memoryResourceName)
	klog.InfoS("Add resource", vCountResourceName, mock.Counts[vCountResourceName], memoryResourceName, mock.Counts[memoryResourceName])
}

func (dev *KunlunVDevices) RunManager() {
	lmock := mock.MockLister{
		ResUpdateChan: make(chan dpm.PluginNameList),
		Heartbeat:     make(chan bool),
		Namespace:     device.GetVendorName(KunlunResourceVCount),
	}
	go func() {
		lmock.ResUpdateChan <- dev.resourceNames
	}()
	mockmanager := dpm.NewManager(&lmock)
	klog.Infof("Running mocking dp: %s", XPUCommonWord)
	mockmanager.Run()
}

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
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	//"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	corev1 "k8s.io/api/core/v1"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
)

const (
	XPUDevice      = "XPU"
	XPUCommonWord  = "XPU"
	RegisterAnnos  = "hami.io/node-register-xpu"
	HandshakeAnnos = "hami.io/node-handshake-xpu"
)

type KunlunConfig struct {
	ResourceCountName   string `yaml:"resourceCountName"`
	ResourceVCountName  string `yaml:"resourceVCountName"`
	ResourceVMemoryName string `yaml:"resourceVMemoryName"`
}

type KunlunVDevices struct {
	config KunlunConfig
}

func InitKunlunVDevice(config KunlunConfig) *KunlunVDevices {
	vCountVendor, vCountResource, vCountQualified := strings.Cut(config.ResourceVCountName, "/")
	vMemoryVendor, vMemoryResource, vMemoryQualified := strings.Cut(config.ResourceVMemoryName, "/")
	if config.ResourceVCountName == "" || config.ResourceVMemoryName == "" ||
		config.ResourceVCountName == config.ResourceVMemoryName ||
		!vCountQualified || !vMemoryQualified ||
		vCountVendor == "" || vMemoryVendor == "" ||
		vCountResource == "" || vMemoryResource == "" ||
		len(utilvalidation.IsQualifiedName(config.ResourceVCountName)) > 0 ||
		len(utilvalidation.IsQualifiedName(config.ResourceVMemoryName)) > 0 ||
		vCountVendor != vMemoryVendor {
		return nil
	}
	return &KunlunVDevices{config: config}
}

func (dev *KunlunVDevices) CommonWord() string {
	return XPUDevice
}

func (dev *KunlunVDevices) GetNodeDevices(n *corev1.Node) ([]*device.DeviceInfo, error) {
	anno, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, fmt.Errorf("annos not found %s", RegisterAnnos)
	}
	nodeDevices, err := device.UnMarshalNodeDevices(anno)
	if err != nil {
		klog.ErrorS(err, "failed to unmarshal node devices", "node", n.Name, "device annotation", anno)
		return []*device.DeviceInfo{}, err
	}
	for idx, nodeDevice := range nodeDevices {
		if nodeDevice == nil {
			return []*device.DeviceInfo{}, fmt.Errorf("device %d in %s is null", idx, RegisterAnnos)
		}
		if nodeDevice.Count < 0 || nodeDevice.Devmem < 0 || nodeDevice.Devcore < 0 {
			return []*device.DeviceInfo{}, fmt.Errorf("device %d in %s has negative capacity", idx, RegisterAnnos)
		}
		nodeDevice.DeviceVendor = dev.CommonWord()
	}
	if len(nodeDevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", anno)
		return []*device.DeviceInfo{}, errors.New("no device found on node")
	}
	return nodeDevices, nil
}

func (dev *KunlunVDevices) GetResource(n *corev1.Node) map[string]int {
	memoryResourceName := device.GetResourceName(dev.config.ResourceVMemoryName)
	vCountResourceName := device.GetResourceName(dev.config.ResourceVCountName)
	resourceMap := map[string]int{
		memoryResourceName: 0,
		vCountResourceName: 0,
	}
	devInfos, err := dev.GetNodeDevices(n)
	if err != nil || len(devInfos) == 0 {
		klog.Infof("no device %s on this node", dev.CommonWord())
		return resourceMap
	}
	for _, val := range devInfos {
		if !val.Health {
			continue
		}
		resourceMap[vCountResourceName] += int(val.Devcore)
		resourceMap[memoryResourceName] += int(val.Devmem)
	}
	klog.InfoS("Add resource", vCountResourceName, resourceMap[vCountResourceName], memoryResourceName, resourceMap[memoryResourceName])
	return resourceMap
}

func (dev *KunlunVDevices) RunManager() {
	lmock := mock.NewMockLister(device.GetVendorName(dev.config.ResourceVCountName))
	go device.Register(lmock, dev)
	mockmanager := dpm.NewManager(lmock)
	klog.Infof("Running mocking dp: %s", dev.CommonWord())
	mockmanager.Run()
}

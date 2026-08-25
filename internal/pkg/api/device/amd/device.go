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

package amd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
)

const (
	AMDDevice     = "AMD"
	AMDCommonWord = "AMD"
	RegisterAnnos = "hami.io/node-amd-register"
)

type AMDConfig struct {
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
	ResourceCoreName   string `yaml:"resourceCoreName"`
}

type AMDDevices struct {
	config AMDConfig
}

func InitAMDGPUDevice(config AMDConfig) (*AMDDevices, error) {
	if config.ResourceCountName == "" && config.ResourceMemoryName == "" && config.ResourceCoreName == "" {
		return nil, nil
	}
	resourceNames := []string{
		config.ResourceCountName,
		config.ResourceMemoryName,
		config.ResourceCoreName,
	}
	for _, resourceName := range resourceNames {
		if resourceName == "" {
			return nil, errors.New("amd resource count, memory, and core names are required")
		}
		vendor, resource, qualified := strings.Cut(resourceName, "/")
		if !qualified || vendor == "" || resource == "" || len(utilvalidation.IsQualifiedName(resourceName)) > 0 {
			return nil, fmt.Errorf("amd resource name %q must be a qualified extended resource name", resourceName)
		}
	}
	if config.ResourceCountName == config.ResourceMemoryName ||
		config.ResourceCountName == config.ResourceCoreName ||
		config.ResourceMemoryName == config.ResourceCoreName {
		return nil, errors.New("amd resource count, memory, and core names must be distinct")
	}
	vendor := device.GetVendorName(config.ResourceCountName)
	if device.GetVendorName(config.ResourceMemoryName) != vendor ||
		device.GetVendorName(config.ResourceCoreName) != vendor {
		return nil, errors.New("amd resource count, memory, and core names must use the same vendor namespace")
	}

	return &AMDDevices{config: config}, nil
}

func (dev *AMDDevices) CommonWord() string {
	return AMDCommonWord
}

func (dev *AMDDevices) GetNodeDevices(n *corev1.Node) ([]*device.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodeDevices, err := device.UnMarshalNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, err
	}
	if len(nodeDevices) == 0 {
		return []*device.DeviceInfo{}, fmt.Errorf("no amd gpu found on node %s", n.Name)
	}
	for idx, nodeDevice := range nodeDevices {
		if nodeDevice == nil {
			return []*device.DeviceInfo{}, fmt.Errorf("amd device annotation contains null entry at index %d", idx)
		}
		if nodeDevice.Count < 0 || nodeDevice.Devmem < 0 || nodeDevice.Devcore < 0 {
			return []*device.DeviceInfo{}, fmt.Errorf("amd device annotation contains negative capacity at index %d", idx)
		}
		nodeDevice.DeviceVendor = dev.CommonWord()
	}
	return nodeDevices, nil
}

func (dev *AMDDevices) GetResource(n *corev1.Node) map[string]int {
	memoryResourceName := device.GetResourceName(dev.config.ResourceMemoryName)
	coreResourceName := device.GetResourceName(dev.config.ResourceCoreName)
	resourceMap := map[string]int{
		memoryResourceName: 0,
		coreResourceName:   0,
	}
	if !device.CheckHealthy(n, dev.config.ResourceCountName) {
		klog.Infof("device %s is unhealthy on this node", dev.CommonWord())
		return resourceMap
	}

	nodeDevices, err := dev.GetNodeDevices(n)
	if err != nil {
		klog.Infof("no device %s on this node", dev.CommonWord())
		return resourceMap
	}
	for _, nodeDevice := range nodeDevices {
		if !nodeDevice.Health {
			continue
		}
		resourceMap[memoryResourceName] += int(nodeDevice.Devmem)
		// HAMi exposes AMD core requests as percentages. The annotation's
		// Devcore remains the physical CU count used by the scheduler to turn
		// a percentage request into CUs; every healthy card contributes 100.
		resourceMap[coreResourceName] += 100
	}
	klog.InfoS("Add resources",
		memoryResourceName, resourceMap[memoryResourceName],
		coreResourceName, resourceMap[coreResourceName],
	)
	return resourceMap
}

func (dev *AMDDevices) RunManager() {
	lmock := mock.NewMockLister(device.GetVendorName(dev.config.ResourceMemoryName))
	go device.Register(lmock, dev)
	mockManager := dpm.NewManager(lmock)
	klog.Infof("Running mocking dp: %s", dev.CommonWord())
	mockManager.Run()
}

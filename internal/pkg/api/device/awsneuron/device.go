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

package awsneuron

import (
	"fmt"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	//"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	//"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

const (
	AWSNeuronDevice          = "AWSNeuron"
	AWSNeuronCommonWord      = "AWSNeuron"
	AWSNeuronDeviceSelection = "aws.amazon.com/neuron-index"
	AWSNeuronUseUUID         = "aws.amazon.com/use-neuron-uuid"
	AWSNeuronNoUseUUID       = "aws.amazon.com/nouse-neuron-uuid"
	AWSNeuronAssignedIndex   = "AWS_NEURON_IDS"
	AWSNeuronAssignedNode    = "aws.amazon.com/predicate-node"
	AWSNeuronPredicateTime   = "NEURON_ALLOC_TIME"
	AWSNeuronResourceType    = "NEURON_RESOURCE_TYPE"
	AWSNeuronAllocated       = "NEURON_ALLOCATED"
	AWSUsageInfo             = "awsusageinfo"
	AWSNodeType              = "AWSNodeType"
)

type AWSNeuronConfig struct {
	ResourceCountName string `yaml:"resourceCountName"`
	ResourceCoreName  string `yaml:"resourceCoreName"`
}

type AWSNeuronDevices struct {
	resourceCountName string
	resourceCoreName  string
	coresPerAWSNeuron uint
	coremask          uint
}

func ParseConfig() {
}

func InitAWSNeuronDevice(config AWSNeuronConfig) *AWSNeuronDevices {
	return &AWSNeuronDevices{
		resourceCountName: config.ResourceCountName,
		resourceCoreName:  config.ResourceCoreName,
		coresPerAWSNeuron: 0,
		coremask:          0,
	}
}

func (dev *AWSNeuronDevices) CommonWord() string {
	return AWSNeuronCommonWord
}

func (dev *AWSNeuronDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	nodedevices := []*device.DeviceInfo{}
	i := 0
	counts, ok := n.Status.Capacity.Name(corev1.ResourceName(dev.resourceCountName), resource.DecimalSI).AsInt64()
	if !ok || counts == 0 {
		return []*device.DeviceInfo{}, fmt.Errorf("device not found %s", dev.resourceCountName)
	}
	coresTotal, _ := n.Status.Capacity.Name(corev1.ResourceName(dev.resourceCoreName), resource.DecimalSI).AsInt64()
	if dev.coresPerAWSNeuron == 0 {
		dev.coresPerAWSNeuron = uint(coresTotal) / uint(counts)
	}
	dev.coremask = 0
	for i < int(dev.coresPerAWSNeuron) {
		dev.coremask *= 2
		dev.coremask++
		i++
	}
	i = 0
	customInfo := map[string]any{}
	customInfo[AWSNodeType] = n.Labels["node.kubernetes.io/instance-type"]

	for int64(i) < counts {
		nodedevices = append(nodedevices, &device.DeviceInfo{
			Index:        uint(i),
			ID:           n.Name + "-" + AWSNeuronDevice + "-" + fmt.Sprint(i),
			Count:        int32(dev.coresPerAWSNeuron),
			Devmem:       0,
			Devcore:      int32(dev.coremask),
			Type:         AWSNeuronDevice,
			Numa:         0,
			Health:       true,
			CustomInfo:   customInfo,
			DeviceVendor: AWSNeuronCommonWord,
		})
		i++
	}
	i = 0
	for i < len(nodedevices) {
		klog.V(4).Infoln("Registered AWS nodedevices:", nodedevices[i])
		i++
	}
	return nodedevices, nil
}

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

package iluvatar

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
)

// MemoryFactor matches HAMi's Iluvatar resource contract: one vMem unit is
// 256 MiB, while the device registration annotation stores memory in MiB.
const MemoryFactor = 256

type IluvatarConfig struct {
	CommonWord         string `yaml:"commonWord"`
	ChipName           string `yaml:"chipName"`
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
	ResourceCoreName   string `yaml:"resourceCoreName"`
}

type IluvatarDevices struct {
	config           IluvatarConfig
	nodeRegisterAnno string
}

// InitIluvatarDevices validates the complete Iluvatar configuration before it
// returns any devices. This keeps configuration failures from silently
// overwriting a device or resource registration.
func InitIluvatarDevices(configs []IluvatarConfig) ([]*IluvatarDevices, error) {
	commonWords := make(map[string]struct{}, len(configs))
	resourceNames := make(map[string]struct{}, len(configs)*3)
	devices := make([]*IluvatarDevices, 0, len(configs))

	for idx, config := range configs {
		if err := validateConfig(config, commonWords, resourceNames); err != nil {
			return nil, fmt.Errorf("invalid iluvatar config at index %d: %w", idx, err)
		}

		commonWords[config.CommonWord] = struct{}{}
		resourceNames[config.ResourceCountName] = struct{}{}
		resourceNames[config.ResourceMemoryName] = struct{}{}
		resourceNames[config.ResourceCoreName] = struct{}{}
		devices = append(devices, &IluvatarDevices{
			config:           config,
			nodeRegisterAnno: fmt.Sprintf("hami.io/node-%s-register", config.CommonWord),
		})
	}

	return devices, nil
}

func validateConfig(config IluvatarConfig, commonWords, resourceNames map[string]struct{}) error {
	if strings.TrimSpace(config.CommonWord) == "" {
		return errors.New("commonWord must not be empty")
	}
	if strings.Contains(config.CommonWord, "/") {
		return fmt.Errorf("commonWord %q must not contain '/'", config.CommonWord)
	}
	if problems := validation.IsQualifiedName(config.CommonWord); len(problems) > 0 {
		return fmt.Errorf("commonWord %q is invalid: %s", config.CommonWord, strings.Join(problems, "; "))
	}
	annotationKey := fmt.Sprintf("hami.io/node-%s-register", config.CommonWord)
	if problems := validation.IsQualifiedName(annotationKey); len(problems) > 0 {
		return fmt.Errorf("commonWord %q produces invalid annotation key %q: %s", config.CommonWord, annotationKey, strings.Join(problems, "; "))
	}
	if strings.TrimSpace(config.ChipName) == "" {
		return errors.New("chipName must not be empty")
	}
	if _, exists := commonWords[config.CommonWord]; exists {
		return fmt.Errorf("duplicate commonWord %q", config.CommonWord)
	}

	resources := []struct {
		field string
		name  string
	}{
		{field: "resourceCountName", name: config.ResourceCountName},
		{field: "resourceMemoryName", name: config.ResourceMemoryName},
		{field: "resourceCoreName", name: config.ResourceCoreName},
	}

	vendor := ""
	entryResources := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if problems := validation.IsQualifiedName(resource.name); len(problems) > 0 {
			return fmt.Errorf("%s %q is not a valid extended-resource name: %s", resource.field, resource.name, strings.Join(problems, "; "))
		}
		resourceVendor, resourceName, ok := strings.Cut(resource.name, "/")
		if !ok || strings.TrimSpace(resourceVendor) == "" || strings.TrimSpace(resourceName) == "" {
			return fmt.Errorf("%s %q must use the extended-resource form vendor/name", resource.field, resource.name)
		}
		if vendor == "" {
			vendor = resourceVendor
		} else if resourceVendor != vendor {
			return fmt.Errorf("%s %q must use vendor namespace %q", resource.field, resource.name, vendor)
		}
		if _, exists := entryResources[resource.name]; exists {
			return fmt.Errorf("duplicate resource name %q", resource.name)
		}
		if _, exists := resourceNames[resource.name]; exists {
			return fmt.Errorf("duplicate resource name %q", resource.name)
		}
		entryResources[resource.name] = struct{}{}
	}

	return nil
}

func (dev *IluvatarDevices) CommonWord() string {
	return dev.config.CommonWord
}

func (dev *IluvatarDevices) GetNodeDevices(n *corev1.Node) ([]*device.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[dev.nodeRegisterAnno]
	if !ok {
		return []*device.DeviceInfo{}, errors.New("annos not found " + dev.nodeRegisterAnno)
	}

	nodeDevices, err := decodeNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, err
	}
	if len(nodeDevices) == 0 {
		klog.InfoS("no iluvatar gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, errors.New("no gpu found on node")
	}
	for idx, nodeDevice := range nodeDevices {
		if nodeDevice.Count < 0 || nodeDevice.Devmem < 0 || nodeDevice.Devcore < 0 {
			return []*device.DeviceInfo{}, fmt.Errorf(
				"device %d has negative capacity: count=%d devmem=%d devcore=%d",
				idx, nodeDevice.Count, nodeDevice.Devmem, nodeDevice.Devcore,
			)
		}
		nodeDevice.DeviceVendor = dev.CommonWord()
	}

	return nodeDevices, nil
}

// decodeNodeDevices follows HAMi's current legacy CSV annotation contract but
// fails closed on malformed fields. The repository's older shared decoder
// silently coerces parse errors to zero, which is unsafe for configured
// Iluvatar resources and would also turn a negative index into a large uint.
func decodeNodeDevices(encoded string) ([]*device.DeviceInfo, error) {
	if !strings.Contains(encoded, device.OneContainerMultiDeviceSplitSymbol) {
		return nil, errors.New("node annotation missing device separator")
	}

	segments := strings.Split(encoded, device.OneContainerMultiDeviceSplitSymbol)
	devices := make([]*device.DeviceInfo, 0, len(segments))
	deviceIDs := make(map[string]struct{}, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if !strings.Contains(segment, ",") {
			return nil, fmt.Errorf("malformed node annotation segment: %q", segment)
		}
		fields := strings.Split(segment, ",")
		if len(fields) != 7 && len(fields) != 9 {
			return nil, fmt.Errorf("unexpected field count %d in node annotation", len(fields))
		}
		if strings.TrimSpace(fields[0]) == "" {
			return nil, errors.New("device ID must not be empty")
		}
		if _, exists := deviceIDs[fields[0]]; exists {
			return nil, fmt.Errorf("duplicate device ID %q", fields[0])
		}
		deviceIDs[fields[0]] = struct{}{}

		count, err := strconv.ParseInt(fields[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid count field: %w", err)
		}
		memory, err := strconv.ParseInt(fields[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid memory field: %w", err)
		}
		core, err := strconv.ParseInt(fields[3], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid core field: %w", err)
		}
		numa, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, fmt.Errorf("invalid numa field: %w", err)
		}
		health, err := strconv.ParseBool(fields[6])
		if err != nil {
			return nil, fmt.Errorf("invalid health field: %w", err)
		}

		index := 0
		mode := "hami-core"
		if len(fields) == 9 {
			index, err = strconv.Atoi(fields[7])
			if err != nil {
				return nil, fmt.Errorf("invalid index field: %w", err)
			}
			if index < 0 {
				return nil, fmt.Errorf("index field must not be negative: %d", index)
			}
			mode = fields[8]
		}

		devices = append(devices, &device.DeviceInfo{
			ID:      fields[0],
			Index:   uint(index),
			Count:   int32(count),
			Devmem:  int32(memory),
			Devcore: int32(core),
			Type:    fields[4],
			Numa:    numa,
			Mode:    mode,
			Health:  health,
		})
	}
	return devices, nil
}

func (dev *IluvatarDevices) GetResource(n *corev1.Node) map[string]int {
	countResourceName := device.GetResourceName(dev.config.ResourceCountName)
	memoryResourceName := device.GetResourceName(dev.config.ResourceMemoryName)
	coreResourceName := device.GetResourceName(dev.config.ResourceCoreName)
	resourceMap := map[string]int{
		countResourceName:  0,
		memoryResourceName: 0,
		coreResourceName:   0,
	}

	nodeDevices, err := dev.GetNodeDevices(n)
	if err != nil {
		klog.Infof("no device %s on this node", dev.CommonWord())
		return resourceMap
	}

	rawMemory := 0
	for _, nodeDevice := range nodeDevices {
		if !nodeDevice.Health {
			continue
		}
		resourceMap[countResourceName] += int(nodeDevice.Count)
		rawMemory += int(nodeDevice.Devmem)
		resourceMap[coreResourceName] += int(nodeDevice.Devcore)
	}
	resourceMap[memoryResourceName] = rawMemory / MemoryFactor

	klog.InfoS("Add resources",
		countResourceName, resourceMap[countResourceName],
		memoryResourceName, resourceMap[memoryResourceName],
		coreResourceName, resourceMap[coreResourceName],
	)
	return resourceMap
}

func (dev *IluvatarDevices) RunManager() {
	lmock := mock.NewMockLister(device.GetVendorName(dev.config.ResourceCountName))
	go device.Register(lmock, dev)
	mockManager := dpm.NewManager(lmock)
	klog.Infof("Running mocking dp: %s", dev.CommonWord())
	mockManager.Run()
}

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

package config

import (
	"testing"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/hygon"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/iluvatar"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/nvidia"
)

func iluvatarConfig(commonWord string) iluvatar.IluvatarConfig {
	return iluvatar.IluvatarConfig{
		CommonWord:         commonWord,
		ChipName:           commonWord,
		ResourceCountName:  "iluvatar.ai/" + commonWord + "-vgpu",
		ResourceMemoryName: "iluvatar.ai/" + commonWord + ".vMem",
		ResourceCoreName:   "iluvatar.ai/" + commonWord + ".vCore",
	}
}

func TestInitDevicesWithConfigRegistersIluvatarDevices(t *testing.T) {
	config := &Config{IluvatarConfig: []iluvatar.IluvatarConfig{
		iluvatarConfig("MR-V100"),
		iluvatarConfig("MR-V50"),
		iluvatarConfig("BI-V150"),
		iluvatarConfig("BI-V100"),
	}}

	if err := InitDevicesWithConfig(config); err != nil {
		t.Fatalf("InitDevicesWithConfig() error = %v", err)
	}
	for _, commonWord := range []string{"MR-V100", "MR-V50", "BI-V150", "BI-V100"} {
		dev, exists := device.DevicesMap[commonWord]
		if !exists {
			t.Errorf("device %q was not registered", commonWord)
			continue
		}
		if dev.CommonWord() != commonWord {
			t.Errorf("device %q CommonWord() = %q", commonWord, dev.CommonWord())
		}
	}
}

func TestInitDevicesWithConfigRejectsIluvatarConstructorCollision(t *testing.T) {
	if err := InitDevicesWithConfig(&Config{}); err != nil {
		t.Fatalf("InitDevicesWithConfig() setup error = %v", err)
	}
	duplicate := iluvatarConfig("MR-V100")
	config := &Config{IluvatarConfig: []iluvatar.IluvatarConfig{duplicate, duplicate}}

	if err := InitDevicesWithConfig(config); err == nil {
		t.Fatal("InitDevicesWithConfig() error = nil, want duplicate-config error")
	}
	if _, exists := device.DevicesMap["MR-V100"]; exists {
		t.Fatal("InitDevicesWithConfig() partially registered Iluvatar devices on constructor error")
	}
}

func TestInitDevicesWithConfigRejectsExistingCommonWordCollision(t *testing.T) {
	initialHygon := hygon.HygonConfig{
		ResourceCountName:  "old.example/dcu",
		ResourceMemoryName: "old.example/dcumem",
		ResourceCoreName:   "old.example/dcucores",
		MemoryFactor:       1,
	}
	if err := InitDevicesWithConfig(&Config{HygonConfig: initialHygon}); err != nil {
		t.Fatalf("InitDevicesWithConfig() setup error = %v", err)
	}
	previousNvidia := device.DevicesMap[nvidia.NvidiaGPUDevice]
	previousLen := len(device.DevicesMap)
	config := &Config{
		HygonConfig: hygon.HygonConfig{
			ResourceCountName:  "new.example/dcu",
			ResourceMemoryName: "new.example/dcumem",
			ResourceCoreName:   "new.example/dcucores",
			MemoryFactor:       2,
		},
		IluvatarConfig: []iluvatar.IluvatarConfig{iluvatarConfig("NVIDIA")},
	}

	if err := InitDevicesWithConfig(config); err == nil {
		t.Fatal("InitDevicesWithConfig() error = nil, want CommonWord collision error")
	}
	if len(device.DevicesMap) != previousLen || device.DevicesMap[nvidia.NvidiaGPUDevice] != previousNvidia {
		t.Fatalf("failed initialization changed DevicesMap: %#v", device.DevicesMap)
	}
	if hygon.HygonResourceCount != initialHygon.ResourceCountName ||
		hygon.HygonResourceMemory != initialHygon.ResourceMemoryName ||
		hygon.HygonResourceCores != initialHygon.ResourceCoreName ||
		hygon.MemoryFactor != initialHygon.MemoryFactor {
		t.Fatalf("failed initialization changed Hygon globals: count=%q memory=%q core=%q factor=%d",
			hygon.HygonResourceCount, hygon.HygonResourceMemory, hygon.HygonResourceCores, hygon.MemoryFactor)
	}
}

func TestInitDevicesWithConfigRejectsIluvatarResourceCollisionAtomically(t *testing.T) {
	if err := InitDevicesWithConfig(&Config{}); err != nil {
		t.Fatalf("initial InitDevicesWithConfig() error = %v", err)
	}
	previousNvidia := device.DevicesMap[nvidia.NvidiaGPUDevice]
	previousLen := len(device.DevicesMap)

	iluvatarConfig := iluvatarConfig("MR-V100")
	iluvatarConfig.ResourceMemoryName = "nvidia.com/gpumem"
	iluvatarConfig.ResourceCountName = "nvidia.com/iluvatar-vgpu"
	iluvatarConfig.ResourceCoreName = "nvidia.com/iluvatar-vcore"
	config := &Config{
		NvidiaConfig: nvidia.NvidiaConfig{ResourceMemoryName: "nvidia.com/gpumem"},
		IluvatarConfig: []iluvatar.IluvatarConfig{
			iluvatarConfig,
		},
	}

	if err := InitDevicesWithConfig(config); err == nil {
		t.Fatal("InitDevicesWithConfig() error = nil, want resource collision error")
	}
	if len(device.DevicesMap) != previousLen || device.DevicesMap[nvidia.NvidiaGPUDevice] != previousNvidia {
		t.Fatalf("failed initialization changed DevicesMap: %#v", device.DevicesMap)
	}
	if _, exists := device.DevicesMap[iluvatarConfig.CommonWord]; exists {
		t.Fatalf("failed initialization registered Iluvatar device %q", iluvatarConfig.CommonWord)
	}
}

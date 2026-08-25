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
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/amd"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/ascend"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/nvidia"
)

func TestInitDevicesWithAMDConfig(t *testing.T) {
	err := InitDevicesWithConfig(&Config{AMDGPUConfig: amd.AMDConfig{
		ResourceCountName:  "amd.com/gpu",
		ResourceMemoryName: "amd.com/gpumem",
		ResourceCoreName:   "amd.com/gpucores",
	}})
	if err != nil {
		t.Fatalf("InitDevicesWithConfig() error = %v", err)
	}
	if _, ok := device.DevicesMap[amd.AMDCommonWord]; !ok {
		t.Fatalf("AMD device missing from DevicesMap: %v", device.DevicesMap)
	}
}

func TestInitDevicesWithoutAMDConfig(t *testing.T) {
	if err := InitDevicesWithConfig(&Config{}); err != nil {
		t.Fatalf("InitDevicesWithConfig() error = %v", err)
	}
	if _, ok := device.DevicesMap[amd.AMDCommonWord]; ok {
		t.Fatalf("disabled AMD device unexpectedly present in DevicesMap: %v", device.DevicesMap)
	}
}

func TestInitDevicesRejectsInvalidAMDConfig(t *testing.T) {
	err := InitDevicesWithConfig(&Config{AMDGPUConfig: amd.AMDConfig{
		ResourceCountName:  "amd.com/gpu",
		ResourceMemoryName: "other.example/gpumem",
		ResourceCoreName:   "amd.com/gpucores",
	}})
	if err == nil {
		t.Fatal("InitDevicesWithConfig() error = nil, want invalid AMD resource namespace error")
	}
}

func TestInitDevicesRejectsAMDCommonWordCollisionAtomically(t *testing.T) {
	if err := InitDevicesWithConfig(&Config{}); err != nil {
		t.Fatalf("InitDevicesWithConfig() setup error = %v", err)
	}
	previousNvidia := device.DevicesMap[nvidia.NvidiaGPUDevice]
	previousLen := len(device.DevicesMap)

	err := InitDevicesWithConfig(&Config{
		AMDGPUConfig: amd.AMDConfig{
			ResourceCountName:  "amd.com/gpu",
			ResourceMemoryName: "amd.com/gpumem",
			ResourceCoreName:   "amd.com/gpucores",
		},
		VNPUs: ascend.VNPUs{Configs: []ascend.VNPUConfig{{
			CommonWord:         amd.AMDCommonWord,
			ResourceName:       "huawei.com/amd",
			ResourceMemoryName: "huawei.com/amd-memory",
		}}},
	})
	if err == nil {
		t.Fatal("InitDevicesWithConfig() error = nil, want duplicate common word error")
	}
	if len(device.DevicesMap) != previousLen || device.DevicesMap[nvidia.NvidiaGPUDevice] != previousNvidia {
		t.Fatalf("failed initialization changed DevicesMap: %v", device.DevicesMap)
	}
	if _, ok := device.DevicesMap[amd.AMDCommonWord]; ok {
		t.Fatalf("failed initialization installed AMD device: %v", device.DevicesMap)
	}
}

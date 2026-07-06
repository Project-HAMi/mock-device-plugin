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
	"os"
	"path/filepath"
	"testing"

	deviceapi "github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/ascend"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "device-config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigLoadsLatestWrappedVNPUs(t *testing.T) {
	path := writeTempConfig(t, `vnpus:
  hamiVnpuCore: true
  configs:
    - commonWord: Ascend910A
      chipName: 910A
      resourceName: huawei.com/Ascend910A
      resourceMemoryName: huawei.com/Ascend910A-memory
      resourceCoreName: huawei.com/Ascend910A-core
      memoryAllocatable: 32768
      memoryCapacity: 32768
      memoryFactor: 1
      aiCore: 20
      aiCPU: 7
      runtimeClassName: hami-ascend
      overwriteEnv: true
      templates:
        - name: vir05_1c_8g
          memory: 8192
          aiCore: 5
          aiCPU: 1
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if !cfg.VNPUs.HamiVnpuCore {
		t.Fatalf("expected HamiVnpuCore=true")
	}
	if len(cfg.VNPUs.Configs) != 1 {
		t.Fatalf("expected 1 VNPU config, got %d", len(cfg.VNPUs.Configs))
	}

	got := cfg.VNPUs.Configs[0]
	if got.ResourceCoreName != "huawei.com/Ascend910A-core" {
		t.Fatalf("unexpected ResourceCoreName: %s", got.ResourceCoreName)
	}
	if got.RuntimeClassName != "hami-ascend" {
		t.Fatalf("unexpected RuntimeClassName: %s", got.RuntimeClassName)
	}
	if !got.OverwriteEnv {
		t.Fatalf("expected OverwriteEnv=true")
	}
}

func TestLoadConfigRejectsFlatVNPUsShape(t *testing.T) {
	path := writeTempConfig(t, `vnpus:
  - commonWord: Ascend910A
    chipName: 910A
    resourceName: huawei.com/Ascend910A
    resourceMemoryName: huawei.com/Ascend910A-memory
    memoryAllocatable: 32768
    memoryCapacity: 32768
    memoryFactor: 1
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatalf("expected LoadConfig to reject flat vnpus shape")
	}
}

func TestInitDevicesWithConfigUsesWrappedConfigs(t *testing.T) {
	t.Cleanup(func() {
		deviceapi.DevicesMap = nil
	})

	cfg := &Config{
		VNPUs: ascend.VNPUs{
			Configs: []ascend.VNPUConfig{
				{
					CommonWord:         "Ascend910A",
					ChipName:           "910A",
					ResourceName:       "huawei.com/Ascend910A",
					ResourceMemoryName: "huawei.com/Ascend910A-memory",
					ResourceCoreName:   "huawei.com/Ascend910A-core",
					MemoryAllocatable:  32768,
					MemoryCapacity:     32768,
					MemoryFactor:       1,
					AICore:             20,
					AICPU:              7,
					RuntimeClassName:   "hami-ascend",
					OverwriteEnv:       true,
					Templates: []ascend.Template{
						{Name: "vir05_1c_8g", Memory: 8192, AICore: 5, AICPU: 1},
					},
				},
			},
		},
	}

	if err := InitDevicesWithConfig(cfg); err != nil {
		t.Fatalf("InitDevicesWithConfig returned error: %v", err)
	}

	if _, ok := deviceapi.DevicesMap["Ascend910A"]; !ok {
		t.Fatalf("expected Ascend910A device to be registered from wrapped configs")
	}
}

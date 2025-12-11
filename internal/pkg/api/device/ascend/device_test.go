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
package ascend

import (
	"testing"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitDevices(t *testing.T) {
	tests := []struct {
		name           string
		configs        []VNPUConfig
		expectedCount  int
		expectedCommon []string
	}{
		{
			name: "single device config",
			configs: []VNPUConfig{
				{
					CommonWord:         "Ascend310P",
					ChipName:           "310P3",
					ResourceName:       "huawei.com/Ascend310P",
					ResourceMemoryName: "huawei.com/Ascend310P-memory",
					MemoryAllocatable:  21527,
					MemoryCapacity:     24576,
					MemoryFactor:       1,
					AICore:             8,
					AICPU:              7,
					Templates: []Template{
						{Name: "vir01", Memory: 3072, AICore: 1, AICPU: 1},
						{Name: "vir02", Memory: 6144, AICore: 2, AICPU: 4},
					},
				},
			},
			expectedCount:  1,
			expectedCommon: []string{"Ascend310P"},
		},
		{
			name: "multiple device configs",
			configs: []VNPUConfig{
				{
					CommonWord:         "Ascend310P",
					ChipName:           "310P3",
					ResourceName:       "huawei.com/Ascend310P",
					ResourceMemoryName: "huawei.com/Ascend310P-memory",
					MemoryAllocatable:  21527,
					MemoryCapacity:     24576,
					MemoryFactor:       1,
					AICore:             8,
					AICPU:              7,
					Templates: []Template{
						{Name: "vir01", Memory: 3072, AICore: 1, AICPU: 1},
						{Name: "vir02", Memory: 6144, AICore: 2, AICPU: 4},
					},
				},
				{
					CommonWord:         "Ascend910B4",
					ChipName:           "910B4",
					ResourceName:       "huawei.com/Ascend910B4",
					ResourceMemoryName: "huawei.com/Ascend910B4-memory",
					MemoryAllocatable:  32768,
					MemoryCapacity:     32768,
					MemoryFactor:       1,
					AICore:             20,
					AICPU:              7,
					Templates: []Template{
						{Name: "vir05_1c_8g", Memory: 8192, AICore: 5, AICPU: 1},
						{Name: "vir10_3c_16g", Memory: 16384, AICore: 10, AICPU: 2},
					},
				},
			},
			expectedCount:  2,
			expectedCommon: []string{"Ascend310P", "Ascend910B4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.Counts = make(map[string]int)

			devices := InitDevices(tt.configs)

			if len(devices) != tt.expectedCount {
				t.Errorf("expected %d devices, got %d", tt.expectedCount, len(devices))
			}

			for i, dev := range devices {
				if tt.expectedCommon[i] != dev.CommonWord() {
					t.Errorf("device %d: expected CommonWord %s, got %s",
						i, tt.expectedCommon[i], dev.CommonWord())
				}

				if len(dev.config.Templates) > 0 {
					for j := 1; j < len(dev.config.Templates); j++ {
						if dev.config.Templates[j-1].Memory > dev.config.Templates[j].Memory {
							t.Errorf("templates are not sorted by memory")
						}
					}
				}

				expectedAnno := "hami.io/node-register-" + tt.expectedCommon[i]
				if dev.nodeRegisterAnno != expectedAnno {
					t.Errorf("expected annotation %s, got %s", expectedAnno, dev.nodeRegisterAnno)
				}
			}
		})
	}
}

func TestAddResource(t *testing.T) {
	testCases := []struct {
		name           string
		memoryFactor   int32
		expectedMemory int
	}{
		{
			name:           "MemoryFactor = 1",
			memoryFactor:   1,
			expectedMemory: 43054, // 21527 * 2 / 1
		},
		{
			name:           "MemoryFactor = 2",
			memoryFactor:   2,
			expectedMemory: 21527, // 21527 * 2 / 2
		},
		{
			name:           "MemoryFactor = 4",
			memoryFactor:   4,
			expectedMemory: 10763, // 21527 * 2 / 4
		},
		{
			name:           "MemoryFactor = 8",
			memoryFactor:   8,
			expectedMemory: 5381, // 21527 * 2 / 8
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock.Counts = make(map[string]int)

			config := VNPUConfig{
				CommonWord:         "Ascend310P",
				ResourceMemoryName: "huawei.com/Ascend310P-memory",
				MemoryFactor:       tc.memoryFactor,
			}

			dev := &Devices{
				config:           config,
				nodeRegisterAnno: "hami.io/node-register-Ascend310P",
				resourceNames:    []string{},
			}

			node := corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Annotations: map[string]string{
						"hami.io/node-register-Ascend310P": `[{"id":"id1","devmem":21527},{"id":"id2","devmem":21527}]`,
					},
				},
			}

			dev.AddResource(node)

			actualMemory := mock.Counts[dev.resourceNames[0]]

			if actualMemory != tc.expectedMemory {
				t.Errorf("MemoryFactor=%d: expected memory %d, got %d",
					tc.memoryFactor, tc.expectedMemory, actualMemory)
			}
		})
	}
}

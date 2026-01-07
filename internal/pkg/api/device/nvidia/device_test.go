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
package nvidia

import (
	"testing"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddResource(t *testing.T) {
	mock.Counts = make(map[string]int)

	// 创建 NVIDIA 设备配置
	config := NvidiaConfig{
		ResourceCountName:            "nvidia.com/gpu",
		ResourceMemoryName:           "nvidia.com/gpu-memory",
		ResourceCoreName:             "nvidia.com/gpu-core",
		ResourceMemoryPercentageName: "nvidia.com/gpu-memory-percentage",
		ResourcePriority:             "nvidia.com/gpu-priority",
		DefaultMemory:                0,
		DefaultCores:                 0,
		DefaultGPUNum:                1,
		OverwriteEnv:                 true,
		DisableCoreLimit:             false,
	}

	dev := InitNvidiaDevice(config)

	// 根据提供的 annotation 创建测试节点
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node-nvidia-a100",
			Annotations: map[string]string{
				RegisterAnnos: `[
				{"id":"GPU-0","index":4,"count":10,"devmem":81920,"devcore":100,"type":"NVIDIA A100-SXM4-80GB","numa":1,"mode":"hami-core","health":true,"devicepairscore":{}},
				{"id":"GPU-1","index":5,"count":10,"devmem":81920,"devcore":100,"type":"NVIDIA A100-SXM4-80GB","numa":1,"mode":"hami-core","health":true,"devicepairscore":{}},
				{"id":"GPU-2","index":6,"count":10,"devmem":81920,"devcore":100,"type":"NVIDIA A100-SXM4-80GB","numa":1,"mode":"hami-core","health":true,"devicepairscore":{}}
				]`,
			},
		},
	}

	t.Run("Test Nvidia A100 device addition", func(t *testing.T) {
		dev.AddResource(node)

		expectedMemoryResource := "gpu-memory"
		expectedCoreResource := "gpu-core"
		expectedMemoryPercentageResource := "gpu-memory-percentage"

		if len(dev.resourceNames) != 3 {
			t.Errorf("expected 3 resource names, got %d: %v", len(dev.resourceNames), dev.resourceNames)
		}

		resourceMap := make(map[string]bool)
		for _, name := range dev.resourceNames {
			resourceMap[name] = true
		}

		if !resourceMap[expectedMemoryResource] {
			t.Errorf("missing memory resource: %s", expectedMemoryResource)
		}
		if !resourceMap[expectedCoreResource] {
			t.Errorf("missing core resource: %s", expectedCoreResource)
		}
		if !resourceMap[expectedMemoryPercentageResource] {
			t.Errorf("missing memory percentage resource: %s", expectedMemoryPercentageResource)
		}

		expectedTotalMemory := 245760
		actualMemory := mock.Counts[expectedMemoryResource]
		if actualMemory != expectedTotalMemory {
			t.Errorf("expected total memory %d, got %d", expectedTotalMemory, actualMemory)
		}

		expectedTotalCore := 300
		actualCore := mock.Counts[expectedCoreResource]
		if actualCore != expectedTotalCore {
			t.Errorf("expected total core %d, got %d", expectedTotalCore, actualCore)
		}

		expectedTotalMemoryPercentage := 300
		actualMemoryPercentage := mock.Counts[expectedMemoryPercentageResource]
		if actualMemoryPercentage != expectedTotalMemoryPercentage {
			t.Errorf("expected total memory percentage %d, got %d", expectedTotalMemoryPercentage, actualMemoryPercentage)
		}

	})
}

func TestGetNodeDevices(t *testing.T) {
	tests := []struct {
		name        string
		setupNode   func() corev1.Node
		wantErr     bool
		wantDevices int
		setupDev    func() *NvidiaGPUDevices
	}{
		{
			name: "no annotation",
			setupNode: func() corev1.Node {
				return corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "node-no-anno",
						Annotations: map[string]string{},
					},
				}
			},
			wantErr:     true,
			wantDevices: 0,
			setupDev: func() *NvidiaGPUDevices {
				return &NvidiaGPUDevices{}
			},
		},
		{
			name: "invalid annotation format",
			setupNode: func() corev1.Node {
				return corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-bad-data",
						Annotations: map[string]string{
							RegisterAnnos: "invalid-data-format",
						},
					},
				}
			},
			wantErr:     true,
			wantDevices: 0,
			setupDev: func() *NvidiaGPUDevices {
				return &NvidiaGPUDevices{}
			},
		},
		{
			name: "empty devices annotation",
			setupNode: func() corev1.Node {
				return corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-empty-devices",
						Annotations: map[string]string{
							RegisterAnnos: "",
						},
					},
				}
			},
			wantErr:     true,
			wantDevices: 0,
			setupDev: func() *NvidiaGPUDevices {
				return &NvidiaGPUDevices{}
			},
		},
		{
			name: "old format",
			setupNode: func() corev1.Node {
				return corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-example",
						Annotations: map[string]string{
							RegisterAnnos: `GPU-f92d2cf4,10,81920,100,NVIDIA-NVIDIA A100-SXM4-80GB,1,true,6,hami-core:GPU-0d5a6e59,10,81920,100,NVIDIA-NVIDIA A100-SXM4-80GB,1,true,4,hami-core:GPU-da197561,10,81920,100,NVIDIA-NVIDIA A100-SXM4-80GB,1,true,5,hami-core:`,
						},
					},
				}
			},
			wantErr:     false,
			wantDevices: 3,
			setupDev: func() *NvidiaGPUDevices {
				return &NvidiaGPUDevices{
					config: NvidiaConfig{},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			node := tt.setupNode()
			dev := tt.setupDev()

			devices, err := dev.GetNodeDevices(node)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodeDevices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(devices) != tt.wantDevices {
				t.Errorf("GetNodeDevices() returned %d devices, want %d", len(devices), tt.wantDevices)
			}

			if !tt.wantErr && len(devices) > 0 {
				for _, d := range devices {
					if d.Devmem == 0 {
						t.Error("Devmem should not be zero")
					}
					if d.Devcore == 0 {
						t.Error("Devcore should not be zero")
					}
				}
			}
		})
	}
}

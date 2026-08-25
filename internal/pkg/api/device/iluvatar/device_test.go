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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testConfig(commonWord string) IluvatarConfig {
	return IluvatarConfig{
		CommonWord:         commonWord,
		ChipName:           commonWord,
		ResourceCountName:  "iluvatar.ai/" + commonWord + "-vgpu",
		ResourceMemoryName: "iluvatar.ai/" + commonWord + ".vMem",
		ResourceCoreName:   "iluvatar.ai/" + commonWord + ".vCore",
	}
}

func mustInitDevice(t *testing.T, config IluvatarConfig) *IluvatarDevices {
	t.Helper()
	devices, err := InitIluvatarDevices([]IluvatarConfig{config})
	if err != nil {
		t.Fatalf("InitIluvatarDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("InitIluvatarDevices() returned %d devices, want 1", len(devices))
	}
	return devices[0]
}

func TestIluvatarDevicesGetNodeDevices(t *testing.T) {
	dev := mustInitDevice(t, testConfig("MR-V100"))

	tests := []struct {
		name       string
		annotation *string
		wantErr    bool
		wantIndex  uint
		wantMode   string
	}{
		{
			name:       "seven fields",
			annotation: stringPointer("GPU-0,10,8192,100,MR-V100,0,true:"),
			wantIndex:  0,
			wantMode:   "hami-core",
		},
		{
			name:       "nine fields",
			annotation: stringPointer("GPU-0,10,8192,100,MR-V100,1,true,3,sriov:"),
			wantIndex:  3,
			wantMode:   "sriov",
		},
		{name: "missing annotation", wantErr: true},
		{name: "empty annotation", annotation: stringPointer(""), wantErr: true},
		{name: "empty device list", annotation: stringPointer(":"), wantErr: true},
		{name: "malformed device", annotation: stringPointer("GPU-0,10,8192:"), wantErr: true},
		{name: "malformed segment", annotation: stringPointer("junk:GPU-0,10,8192,100,MR-V100,0,true:"), wantErr: true},
		{name: "invalid count", annotation: stringPointer("GPU-0,oops,8192,100,MR-V100,0,true:"), wantErr: true},
		{name: "invalid memory", annotation: stringPointer("GPU-0,10,oops,100,MR-V100,0,true:"), wantErr: true},
		{name: "invalid core", annotation: stringPointer("GPU-0,10,8192,oops,MR-V100,0,true:"), wantErr: true},
		{name: "invalid numa", annotation: stringPointer("GPU-0,10,8192,100,MR-V100,oops,true:"), wantErr: true},
		{name: "invalid health", annotation: stringPointer("GPU-0,10,8192,100,MR-V100,0,oops:"), wantErr: true},
		{name: "invalid index", annotation: stringPointer("GPU-0,10,8192,100,MR-V100,0,true,oops,hami-core:"), wantErr: true},
		{name: "negative index", annotation: stringPointer("GPU-0,10,8192,100,MR-V100,0,true,-1,hami-core:"), wantErr: true},
		{name: "empty device ID", annotation: stringPointer(",10,8192,100,MR-V100,0,true:"), wantErr: true},
		{name: "duplicate device ID", annotation: stringPointer("GPU-0,10,8192,100,MR-V100,0,true:GPU-0,10,8192,100,MR-V100,0,true:"), wantErr: true},
		{name: "negative count", annotation: stringPointer("GPU-0,-1,8192,100,MR-V100,0,true:"), wantErr: true},
		{name: "negative memory", annotation: stringPointer("GPU-0,10,-1,100,MR-V100,0,true:"), wantErr: true},
		{name: "negative core", annotation: stringPointer("GPU-0,10,8192,-1,MR-V100,0,true:"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{}
			if tt.annotation != nil {
				annotations[dev.nodeRegisterAnno] = *tt.annotation
			}
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Annotations: annotations}}

			got, err := dev.GetNodeDevices(node)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetNodeDevices() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if len(got) != 0 {
					t.Fatalf("GetNodeDevices() returned %d devices on error", len(got))
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("GetNodeDevices() returned %d devices, want 1", len(got))
			}
			if got[0].ID != "GPU-0" || got[0].Count != 10 || got[0].Devmem != 8192 || got[0].Devcore != 100 {
				t.Fatalf("GetNodeDevices() decoded unexpected device: %+v", got[0])
			}
			if got[0].Index != tt.wantIndex || got[0].Mode != tt.wantMode {
				t.Fatalf("GetNodeDevices() index/mode = %d/%q, want %d/%q", got[0].Index, got[0].Mode, tt.wantIndex, tt.wantMode)
			}
			if got[0].DeviceVendor != "MR-V100" || !got[0].Health {
				t.Fatalf("GetNodeDevices() vendor/health = %q/%v", got[0].DeviceVendor, got[0].Health)
			}
		})
	}
}

func TestIluvatarDevicesGetResource(t *testing.T) {
	config := testConfig("MR-V100")
	dev := mustInitDevice(t, config)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: "node-1",
		Annotations: map[string]string{
			dev.nodeRegisterAnno: strings.Join([]string{
				"GPU-0,2,255,40,MR-V100,0,true,0,hami-core",
				"GPU-1,3,257,60,MR-V100,0,true,1,hami-core",
				"GPU-2,99,8192,100,MR-V100,0,false,2,hami-core",
			}, ":") + ":",
		},
	}}

	got := dev.GetResource(node)
	if got["MR-V100-vgpu"] != 5 {
		t.Errorf("count = %d, want 5", got["MR-V100-vgpu"])
	}
	// The division applies after summing: floor((255 + 257) / 256) = 2.
	if got["MR-V100.vMem"] != 2 {
		t.Errorf("memory = %d, want 2", got["MR-V100.vMem"])
	}
	if got["MR-V100.vCore"] != 100 {
		t.Errorf("core = %d, want 100", got["MR-V100.vCore"])
	}
}

func TestIluvatarDevicesGetResourceWithoutUsableAnnotation(t *testing.T) {
	dev := mustInitDevice(t, testConfig("MR-V100"))
	tests := []struct {
		name        string
		annotations map[string]string
	}{
		{name: "missing annotation", annotations: map[string]string{}},
		{name: "malformed annotation", annotations: map[string]string{dev.nodeRegisterAnno: "malformed"}},
		{name: "empty annotation", annotations: map[string]string{dev.nodeRegisterAnno: ":"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dev.GetResource(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations}})
			if len(got) != 3 || got["MR-V100-vgpu"] != 0 || got["MR-V100.vMem"] != 0 || got["MR-V100.vCore"] != 0 {
				t.Fatalf("GetResource() = %v, want three zero-valued resources", got)
			}
		})
	}
}

func TestInitIluvatarDevices(t *testing.T) {
	validConfigs := []IluvatarConfig{
		testConfig("MR-V100"),
		testConfig("MR-V50"),
		testConfig("BI-V150"),
		testConfig("BI-V100"),
	}
	devices, err := InitIluvatarDevices(validConfigs)
	if err != nil {
		t.Fatalf("InitIluvatarDevices(valid) error = %v", err)
	}
	if len(devices) != 4 {
		t.Fatalf("InitIluvatarDevices(valid) returned %d devices, want 4", len(devices))
	}
	for idx, dev := range devices {
		if dev.CommonWord() != validConfigs[idx].CommonWord {
			t.Errorf("device %d CommonWord() = %q, want %q", idx, dev.CommonWord(), validConfigs[idx].CommonWord)
		}
		wantAnnotation := "hami.io/node-" + validConfigs[idx].CommonWord + "-register"
		if dev.nodeRegisterAnno != wantAnnotation {
			t.Errorf("device %d annotation = %q, want %q", idx, dev.nodeRegisterAnno, wantAnnotation)
		}
	}

	if empty, err := InitIluvatarDevices(nil); err != nil || len(empty) != 0 {
		t.Fatalf("InitIluvatarDevices(nil) = %v, %v; want empty, nil", empty, err)
	}
}

func TestInitIluvatarDevicesRejectsInvalidConfig(t *testing.T) {
	valid := testConfig("MR-V100")
	tests := []struct {
		name    string
		configs []IluvatarConfig
	}{
		{name: "missing commonWord", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.CommonWord = "" })}},
		{name: "invalid commonWord", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.CommonWord = "bad/word" })}},
		{name: "annotation key too long", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.CommonWord = strings.Repeat("a", 63) })}},
		{name: "missing chipName", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.ChipName = "" })}},
		{name: "resource without vendor", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.ResourceCountName = "vgpu" })}},
		{name: "invalid resource name", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.ResourceCountName = "iluvatar.ai/not valid" })}},
		{name: "mixed vendor namespaces", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.ResourceCoreName = "other.example/vcore" })}},
		{name: "duplicate resource within config", configs: []IluvatarConfig{changedConfig(valid, func(c *IluvatarConfig) { c.ResourceCoreName = c.ResourceMemoryName })}},
		{name: "duplicate commonWord", configs: []IluvatarConfig{valid, changedConfig(testConfig("MR-V50"), func(c *IluvatarConfig) { c.CommonWord = valid.CommonWord })}},
		{name: "duplicate resource across configs", configs: []IluvatarConfig{valid, changedConfig(testConfig("MR-V50"), func(c *IluvatarConfig) { c.ResourceCoreName = valid.ResourceCoreName })}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devices, err := InitIluvatarDevices(tt.configs)
			if err == nil {
				t.Fatalf("InitIluvatarDevices() = %v, nil; want validation error", devices)
			}
			if devices != nil {
				t.Fatalf("InitIluvatarDevices() returned partial devices on error: %v", devices)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}

func changedConfig(config IluvatarConfig, change func(*IluvatarConfig)) IluvatarConfig {
	change(&config)
	return config
}

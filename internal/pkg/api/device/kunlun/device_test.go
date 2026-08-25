/*
Copyright 2026 The HAMi Authors.

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testConfig() KunlunConfig {
	return KunlunConfig{
		ResourceCountName:   "kunlunxin.com/xpu",
		ResourceVCountName:  "kunlunxin.com/vxpu",
		ResourceVMemoryName: "kunlunxin.com/vxpu-memory",
	}
}

func TestInitKunlunVDevice(t *testing.T) {
	tests := []struct {
		name   string
		config KunlunConfig
		valid  bool
	}{
		{name: "complete", config: testConfig(), valid: true},
		{
			name: "physical count is optional",
			config: KunlunConfig{
				ResourceVCountName:  "kunlunxin.com/vxpu",
				ResourceVMemoryName: "kunlunxin.com/vxpu-memory",
			},
			valid: true,
		},
		{
			name: "missing virtual count",
			config: KunlunConfig{
				ResourceVMemoryName: "kunlunxin.com/vxpu-memory",
			},
		},
		{
			name: "missing virtual memory",
			config: KunlunConfig{
				ResourceVCountName: "kunlunxin.com/vxpu",
			},
		},
		{
			name: "unqualified resources",
			config: KunlunConfig{
				ResourceVCountName:  "vxpu",
				ResourceVMemoryName: "vxpu-memory",
			},
		},
		{
			name: "invalid uppercase vendor",
			config: KunlunConfig{
				ResourceVCountName:  "KUNLUNXIN.COM/vxpu",
				ResourceVMemoryName: "KUNLUNXIN.COM/vxpu-memory",
			},
		},
		{
			name: "multiple resource separators",
			config: KunlunConfig{
				ResourceVCountName:  "kunlunxin.com/vxpu/bad",
				ResourceVMemoryName: "kunlunxin.com/vxpu-memory",
			},
		},
		{
			name: "different resource vendors",
			config: KunlunConfig{
				ResourceVCountName:  "kunlunxin.com/vxpu",
				ResourceVMemoryName: "example.com/vxpu-memory",
			},
		},
		{
			name: "same resource twice",
			config: KunlunConfig{
				ResourceVCountName:  "kunlunxin.com/vxpu",
				ResourceVMemoryName: "kunlunxin.com/vxpu",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := InitKunlunVDevice(test.config)
			if test.valid {
				if dev == nil {
					t.Fatal("expected device to initialize")
				}
				if dev.config != test.config {
					t.Fatalf("stored config = %#v, want %#v", dev.config, test.config)
				}
				return
			}
			if dev != nil {
				t.Fatalf("expected invalid config to be ignored, got %#v", dev)
			}
		})
	}
}

func TestKunlunVDevicesConfigurationIsPerInstance(t *testing.T) {
	first := InitKunlunVDevice(testConfig())
	second := InitKunlunVDevice(KunlunConfig{
		ResourceVCountName:  "example.com/second-count",
		ResourceVMemoryName: "example.com/second-memory",
	})
	node := nodeWithAnnotation(`[{"id":"XPU-0","devmem":24576,"devcore":1,"type":"XPU","health":true}]`)

	firstResources := first.GetResource(node)
	if got := firstResources["vxpu"]; got != 1 {
		t.Fatalf("first instance count = %d, want 1", got)
	}
	if got := firstResources["vxpu-memory"]; got != 24576 {
		t.Fatalf("first instance memory = %d, want 24576", got)
	}
	if _, exists := firstResources["second-count"]; exists {
		t.Fatalf("first instance used second instance config: %#v", firstResources)
	}

	secondResources := second.GetResource(node)
	if got := secondResources["second-count"]; got != 1 {
		t.Fatalf("second instance count = %d, want 1", got)
	}
	if got := secondResources["second-memory"]; got != 24576 {
		t.Fatalf("second instance memory = %d, want 24576", got)
	}
}

func TestKunlunVDevicesGetNodeDevices(t *testing.T) {
	dev := InitKunlunVDevice(testConfig())
	tests := []struct {
		name       string
		annotation *string
		wantErr    bool
	}{
		{name: "missing", wantErr: true},
		{name: "malformed", annotation: stringPointer("not-json"), wantErr: true},
		{name: "empty", annotation: stringPointer("[]"), wantErr: true},
		{name: "null entry", annotation: stringPointer("[null]"), wantErr: true},
		{
			name:       "negative count",
			annotation: stringPointer(`[{"id":"XPU-0","count":-1,"devmem":24576,"devcore":1,"health":true}]`),
			wantErr:    true,
		},
		{
			name:       "negative memory",
			annotation: stringPointer(`[{"id":"XPU-0","count":1,"devmem":-1,"devcore":1,"health":true}]`),
			wantErr:    true,
		},
		{
			name:       "negative core",
			annotation: stringPointer(`[{"id":"XPU-0","count":1,"devmem":24576,"devcore":-1,"health":true}]`),
			wantErr:    true,
		},
		{
			name:       "valid",
			annotation: stringPointer(`[{"id":"XPU-0","devmem":98304,"devcore":1,"type":"XPU","health":true}]`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "kunlun-node"}}
			if test.annotation != nil {
				node.Annotations = map[string]string{RegisterAnnos: *test.annotation}
			}

			devices, err := dev.GetNodeDevices(node)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got devices %#v", devices)
				}
				if len(devices) != 0 {
					t.Fatalf("devices = %#v, want empty", devices)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetNodeDevices returned error: %v", err)
			}
			if len(devices) != 1 {
				t.Fatalf("device count = %d, want 1", len(devices))
			}
			if devices[0].DeviceVendor != XPUCommonWord {
				t.Fatalf("device vendor = %q, want %q", devices[0].DeviceVendor, XPUCommonWord)
			}
		})
	}
}

func TestKunlunVDevicesGetResourceUsesHealthyAnnotationUnits(t *testing.T) {
	dev := InitKunlunVDevice(testConfig())
	node := nodeWithAnnotation(`[
		{"id":"XPU-0","devmem":24576,"devcore":1,"type":"XPU","health":true},
		{"id":"XPU-1","index":1,"devmem":49152,"devcore":2,"type":"XPU","health":true},
		{"id":"XPU-2","index":2,"devmem":98304,"devcore":4,"type":"XPU","health":false}
	]`)

	resources := dev.GetResource(node)
	if got, want := resources["vxpu"], 3; got != want {
		t.Fatalf("vxpu capacity = %d, want %d", got, want)
	}
	if got, want := resources["vxpu-memory"], 73728; got != want {
		t.Fatalf("vxpu-memory capacity = %d, want %d", got, want)
	}
}

func TestKunlunVDevicesGetResourceInvalidAnnotationReturnsZeroes(t *testing.T) {
	dev := InitKunlunVDevice(testConfig())
	for _, annotation := range []string{"not-json", "[]", "[null]"} {
		resources := dev.GetResource(nodeWithAnnotation(annotation))
		if got := resources["vxpu"]; got != 0 {
			t.Fatalf("annotation %q: vxpu capacity = %d, want 0", annotation, got)
		}
		if got := resources["vxpu-memory"]; got != 0 {
			t.Fatalf("annotation %q: vxpu-memory capacity = %d, want 0", annotation, got)
		}
	}
}

func nodeWithAnnotation(annotation string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "kunlun-node",
		Annotations: map[string]string{RegisterAnnos: annotation},
	}}
}

func stringPointer(value string) *string {
	return &value
}

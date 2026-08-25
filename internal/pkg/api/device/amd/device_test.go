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
	"reflect"
	"testing"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ device.Devices = (*AMDDevices)(nil)

func validAMDConfig() AMDConfig {
	return AMDConfig{
		ResourceCountName:  "amd.com/gpu",
		ResourceMemoryName: "amd.com/gpumem",
		ResourceCoreName:   "amd.com/gpucores",
	}
}

func mustInitAMD(t *testing.T, config AMDConfig) *AMDDevices {
	t.Helper()
	dev, err := InitAMDGPUDevice(config)
	if err != nil {
		t.Fatalf("InitAMDGPUDevice() error = %v", err)
	}
	if dev == nil {
		t.Fatal("InitAMDGPUDevice() returned nil")
	}
	return dev
}

func TestInitAMDGPUDevice(t *testing.T) {
	tests := []struct {
		name      string
		config    AMDConfig
		wantNil   bool
		wantError bool
	}{
		{name: "disabled when config is empty", wantNil: true},
		{name: "valid", config: validAMDConfig()},
		{
			name: "missing core resource",
			config: AMDConfig{
				ResourceCountName:  "amd.com/gpu",
				ResourceMemoryName: "amd.com/gpumem",
			},
			wantError: true,
		},
		{
			name: "mixed vendor namespaces",
			config: AMDConfig{
				ResourceCountName:  "amd.com/gpu",
				ResourceMemoryName: "example.com/gpumem",
				ResourceCoreName:   "amd.com/gpucores",
			},
			wantError: true,
		},
		{
			name: "unqualified resource name",
			config: AMDConfig{
				ResourceCountName:  "amd.com",
				ResourceMemoryName: "amd.com/gpumem",
				ResourceCoreName:   "amd.com/gpucores",
			},
			wantError: true,
		},
		{
			name: "invalid qualified resource name",
			config: AMDConfig{
				ResourceCountName:  "AMD.COM/gpu",
				ResourceMemoryName: "AMD.COM/gpumem",
				ResourceCoreName:   "AMD.COM/gpucores",
			},
			wantError: true,
		},
		{
			name: "duplicate resource names",
			config: AMDConfig{
				ResourceCountName:  "amd.com/gpu",
				ResourceMemoryName: "amd.com/gpu",
				ResourceCoreName:   "amd.com/gpucores",
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev, err := InitAMDGPUDevice(test.config)
			if (err != nil) != test.wantError {
				t.Fatalf("InitAMDGPUDevice() error = %v, wantError %v", err, test.wantError)
			}
			if test.wantError {
				return
			}
			if (dev == nil) != test.wantNil {
				t.Fatalf("InitAMDGPUDevice() nil = %v, want %v", dev == nil, test.wantNil)
			}
			if dev != nil && dev.CommonWord() != AMDCommonWord {
				t.Fatalf("CommonWord() = %q, want %q", dev.CommonWord(), AMDCommonWord)
			}
		})
	}
}

func TestAMDDevicesGetNodeDevices(t *testing.T) {
	dev := mustInitAMD(t, validAMDConfig())
	tests := []struct {
		name        string
		annotations map[string]string
		wantError   bool
	}{
		{
			name: "valid",
			annotations: map[string]string{
				RegisterAnnos: `[{"id":"AMD-MOCK-0","count":2,"devmem":196608,"devcore":304,"type":"AMD-MI300X","health":true}]`,
			},
		},
		{name: "missing annotation", wantError: true},
		{name: "malformed annotation", annotations: map[string]string{RegisterAnnos: `{`}, wantError: true},
		{name: "empty annotation", annotations: map[string]string{RegisterAnnos: `[]`}, wantError: true},
		{name: "null entry", annotations: map[string]string{RegisterAnnos: `[null]`}, wantError: true},
		{name: "negative count", annotations: map[string]string{RegisterAnnos: `[{"count":-1}]`}, wantError: true},
		{name: "negative memory", annotations: map[string]string{RegisterAnnos: `[{"devmem":-1}]`}, wantError: true},
		{name: "negative core", annotations: map[string]string{RegisterAnnos: `[{"devcore":-1}]`}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Annotations: test.annotations}}
			got, err := dev.GetNodeDevices(node)
			if (err != nil) != test.wantError {
				t.Fatalf("GetNodeDevices() error = %v, wantError %v", err, test.wantError)
			}
			if test.wantError {
				return
			}
			if len(got) != 1 {
				t.Fatalf("GetNodeDevices() returned %d devices, want 1", len(got))
			}
			if got[0].DeviceVendor != AMDCommonWord {
				t.Fatalf("DeviceVendor = %q, want %q", got[0].DeviceVendor, AMDCommonWord)
			}
		})
	}
}

func TestAMDDevicesGetResource(t *testing.T) {
	config := validAMDConfig()
	dev := mustInitAMD(t, config)
	annotation := `[` +
		`{"id":"AMD-MOCK-0","count":2,"devmem":196608,"devcore":304,"type":"AMD-MI300X","health":true},` +
		`{"id":"AMD-MOCK-1","count":2,"devmem":128000,"devcore":120,"type":"AMD-MI250","health":true},` +
		`{"id":"AMD-MOCK-2","count":2,"devmem":64000,"devcore":64,"type":"AMD-MOCK","health":false}` +
		`]`

	tests := []struct {
		name       string
		capacity   corev1.ResourceList
		annotation string
		wantMemory int
		wantCore   int
	}{
		{
			name: "healthy cards contribute memory and one hundred core units each",
			capacity: corev1.ResourceList{
				corev1.ResourceName(config.ResourceCountName): resource.MustParse("1"),
			},
			annotation: annotation,
			wantMemory: 324608,
			wantCore:   200,
		},
		{
			name:       "missing external count gate",
			annotation: annotation,
		},
		{
			name: "zero external count gate",
			capacity: corev1.ResourceList{
				corev1.ResourceName(config.ResourceCountName): resource.MustParse("0"),
			},
			annotation: annotation,
		},
		{
			name: "malformed annotation fails closed",
			capacity: corev1.ResourceList{
				corev1.ResourceName(config.ResourceCountName): resource.MustParse("1"),
			},
			annotation: `{`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "node-1",
					Annotations: map[string]string{RegisterAnnos: test.annotation},
				},
				Status: corev1.NodeStatus{Capacity: test.capacity},
			}
			got := dev.GetResource(node)
			if got["gpumem"] != test.wantMemory {
				t.Fatalf("gpumem = %d, want %d", got["gpumem"], test.wantMemory)
			}
			if got["gpucores"] != test.wantCore {
				t.Fatalf("gpucores = %d, want %d", got["gpucores"], test.wantCore)
			}
			if len(got) != 2 {
				t.Fatalf("GetResource() returned %d resources, want 2", len(got))
			}
		})
	}
}

func TestAMDDevicesKeepConfigPerInstance(t *testing.T) {
	first := mustInitAMD(t, validAMDConfig())
	second := mustInitAMD(t, AMDConfig{
		ResourceCountName:  "example.com/cards",
		ResourceMemoryName: "example.com/vram",
		ResourceCoreName:   "example.com/vcores",
	})
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Annotations: map[string]string{
				RegisterAnnos: `[{"id":"AMD-MOCK-0","devmem":1024,"devcore":64,"health":true}]`,
			},
		},
		Status: corev1.NodeStatus{Capacity: corev1.ResourceList{
			"amd.com/gpu":       resource.MustParse("1"),
			"example.com/cards": resource.MustParse("1"),
		}},
	}

	if got, want := first.GetResource(node), map[string]int{"gpumem": 1024, "gpucores": 100}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first GetResource() = %v", got)
	}
	if got, want := second.GetResource(node), map[string]int{"vram": 1024, "vcores": 100}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second GetResource() = %v", got)
	}
}

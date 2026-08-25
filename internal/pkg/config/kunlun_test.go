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

package config

import (
	"testing"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/ascend"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/kunlun"
)

func TestInitDevicesWithConfigAddsKunlun(t *testing.T) {
	t.Cleanup(func() { device.DevicesMap = nil })
	config := &Config{KunlunConfig: kunlun.KunlunConfig{
		ResourceCountName:   "kunlunxin.com/xpu",
		ResourceVCountName:  "kunlunxin.com/vxpu",
		ResourceVMemoryName: "kunlunxin.com/vxpu-memory",
	}}

	if err := InitDevicesWithConfig(config); err != nil {
		t.Fatalf("InitDevicesWithConfig returned error: %v", err)
	}
	if _, exists := device.DevicesMap[kunlun.XPUCommonWord]; !exists {
		t.Fatalf("Kunlun device %q was not initialized", kunlun.XPUCommonWord)
	}
}

func TestInitDevicesWithConfigSkipsIncompleteKunlun(t *testing.T) {
	t.Cleanup(func() { device.DevicesMap = nil })
	config := &Config{KunlunConfig: kunlun.KunlunConfig{
		ResourceVCountName: "kunlunxin.com/vxpu",
	}}

	if err := InitDevicesWithConfig(config); err != nil {
		t.Fatalf("InitDevicesWithConfig returned error: %v", err)
	}
	if _, exists := device.DevicesMap[kunlun.XPUCommonWord]; exists {
		t.Fatalf("incomplete Kunlun config initialized device %q", kunlun.XPUCommonWord)
	}
}

func TestInitDevicesWithConfigRejectsKunlunCommonWordCollision(t *testing.T) {
	t.Cleanup(func() { device.DevicesMap = nil })
	config := &Config{
		VNPUs: ascend.VNPUs{Configs: []ascend.VNPUConfig{{
			CommonWord:         kunlun.XPUCommonWord,
			ResourceName:       "example.com/xpu",
			ResourceMemoryName: "example.com/xpu-memory",
		}}},
		KunlunConfig: kunlun.KunlunConfig{
			ResourceVCountName:  "kunlunxin.com/vxpu",
			ResourceVMemoryName: "kunlunxin.com/vxpu-memory",
		},
	}

	if err := InitDevicesWithConfig(config); err == nil {
		t.Fatal("expected duplicate common word to be rejected")
	}
}

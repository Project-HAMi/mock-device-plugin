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

package mock

import (
	"slices"
	"testing"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
)

func TestSetResourceDiscoversResourceNameChanges(t *testing.T) {
	lister := NewMockLister("example.com")
	lister.ResUpdateChan = make(chan dpm.PluginNameList, 1)

	lister.SetResource(map[string]int{"memory": 4})
	assertResourceUpdate(t, lister.ResUpdateChan, dpm.PluginNameList{"memory"})

	memoryPlugin := lister.NewPlugin("memory").(*MockPlugin)
	lister.SetResource(map[string]int{"memory": 8, "core": 100})
	assertResourceUpdate(t, lister.ResUpdateChan, dpm.PluginNameList{"core", "memory"})
	if got := memoryPlugin.GetCount(); got != 8 {
		t.Fatalf("memory plugin count = %d, want 8", got)
	}

	corePlugin := lister.NewPlugin("core").(*MockPlugin)
	if got := corePlugin.GetCount(); got != 100 {
		t.Fatalf("core plugin count = %d, want 100", got)
	}

	lister.SetResource(map[string]int{"memory": 16, "core": 200})
	assertNoResourceUpdate(t, lister.ResUpdateChan)
	if got := memoryPlugin.GetCount(); got != 16 {
		t.Fatalf("memory plugin count = %d, want 16", got)
	}
	if got := corePlugin.GetCount(); got != 200 {
		t.Fatalf("core plugin count = %d, want 200", got)
	}

	lister.SetResource(map[string]int{"memory": 16})
	assertResourceUpdate(t, lister.ResUpdateChan, dpm.PluginNameList{"memory"})

	lister.SetResource(map[string]int{})
	assertResourceUpdate(t, lister.ResUpdateChan, dpm.PluginNameList{})
}

func TestSetResourceWaitsForPositiveInitialCount(t *testing.T) {
	lister := NewMockLister("example.com")
	lister.ResUpdateChan = make(chan dpm.PluginNameList, 1)

	lister.SetResource(map[string]int{"memory": 0, "core": 0})
	assertNoResourceUpdate(t, lister.ResUpdateChan)

	lister.SetResource(map[string]int{"memory": 4, "core": 0})
	assertResourceUpdate(t, lister.ResUpdateChan, dpm.PluginNameList{"core", "memory"})
}

func assertResourceUpdate(t *testing.T, updates <-chan dpm.PluginNameList, want dpm.PluginNameList) {
	t.Helper()
	select {
	case got := <-updates:
		if !slices.Equal(got, want) {
			t.Fatalf("resource update = %v, want %v", got, want)
		}
	default:
		t.Fatalf("resource update = none, want %v", want)
	}
}

func assertNoResourceUpdate(t *testing.T, updates <-chan dpm.PluginNameList) {
	t.Helper()
	select {
	case got := <-updates:
		t.Fatalf("unexpected resource update: %v", got)
	default:
	}
}

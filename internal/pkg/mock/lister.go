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

package mock

import (
	"sync"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
)

// Lister serves as an interface between imlementation and Manager machinery. User passes
// implementation of this interface to NewManager function. Manager will use it to obtain resource
// namespace, monitor available resources and instantate a new plugin for them.
type MockLister struct {
	ResUpdateChan chan dpm.PluginNameList
	Heartbeat     chan bool
	Namespace     string
	counts        map[string]int
	pluginsMap    map[string]*MockPlugin
	mutex         sync.Mutex
}

func NewMockLister(namespace string) *MockLister {
	return &MockLister{
		ResUpdateChan: make(chan dpm.PluginNameList),
		Heartbeat:     make(chan bool),
		Namespace:     namespace,
		counts:        make(map[string]int),
		pluginsMap:    make(map[string]*MockPlugin),
	}
}

// GetResourceNamespace must return namespace (vendor ID) of implemented Lister. e.g. for
// resources in format "color.example.com/<color>" that would be "color.example.com".
func (l *MockLister) GetResourceNamespace() string {
	return l.Namespace
}

// Discover notifies manager with a list of currently available resources in its namespace.
// e.g. if "color.example.com/red" and "color.example.com/blue" are available in the system,
// it would pass PluginNameList{"red", "blue"} to given channel. In case list of
// resources is static, it would use the channel only once and then return. In case the list is
// dynamic, it could block and pass a new list each times resources changed. If blocking is
// used, it should check whether the channel is closed, i.e. Discover should stop.
func (l *MockLister) Discover(pluginListCh chan dpm.PluginNameList) {
	for {
		select {
		case newResourcesList := <-l.ResUpdateChan: // New resources found
			pluginListCh <- newResourcesList
		case <-pluginListCh: // Stop message received
			// Stop resourceUpdateCh
			return
		}
	}
}

// NewPlugin instantiates a plugin implementation. It is given the last name of the resource,
// e.g. for resource name "color.example.com/red" that would be "red". It must return valid
// implementation of a PluginInterface.
func (l *MockLister) NewPlugin(resourceLastName string) dpm.PluginInterface {
	mockPlugin := MockPlugin{
		ManagedResource: resourceLastName,
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	mockPlugin.SetCount(l.counts[resourceLastName])
	l.pluginsMap[resourceLastName] = &mockPlugin
	return &mockPlugin
}

func (l *MockLister) SetResource(resourceMap map[string]int) {
	if len(resourceMap) == 0 {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.counts = resourceMap
	pluginNums := len(l.pluginsMap)

	if pluginNums == 0 {
		resourceNames := make([]string, 0, len(resourceMap))
		hasNoZeroValue := false
		for name, val := range resourceMap {
			resourceNames = append(resourceNames, name)
			if val > 0 {
				hasNoZeroValue = true
			}
		}
		if hasNoZeroValue {
			l.ResUpdateChan <- resourceNames
		}
	} else {
		for resourceName, val := range resourceMap {
			if plugin, exists := l.pluginsMap[resourceName]; exists {
				plugin.SetCount(val)
			}
		}
	}
}

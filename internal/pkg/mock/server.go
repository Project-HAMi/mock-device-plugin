/*
Copyright 2024 The HAMi Authors.

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
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
	kubeletdevicepluginv1beta1 "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Plugin is identical to DevicePluginServer interface of device plugin API.
type MockPlugin struct {
	ManagedResource string
	Count           int
}

// Start is an optional interface that could be implemented by plugin.
// If case Start is implemented, it will be executed by Manager after
// plugin instantiation and before its registration to kubelet. This
// method could be used to prepare resources before they are offered
// to Kubernetes.
func (p *MockPlugin) Start() error {
	klog.Infoln("Mock manager start")
	return nil
}

// Stop is an optional interface that could be implemented by plugin.
// If case Stop is implemented, it will be executed by Manager after the
// plugin is unregistered from kubelet. This method could be used to tear
// down resources.
func (p *MockPlugin) Stop() error {
	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (p *MockPlugin) GetDevicePluginOptions(ctx context.Context, e *kubeletdevicepluginv1beta1.Empty) (*kubeletdevicepluginv1beta1.DevicePluginOptions, error) {
	return &kubeletdevicepluginv1beta1.DevicePluginOptions{}, nil
}

// PreStartContainer is expected to be called before each container start if indicated by plugin during registration phase.
// PreStartContainer allows kubelet to pass reinitialized devices to containers.
// PreStartContainer allows Device Plugin to run device specific operations on the Devices requested
func (p *MockPlugin) PreStartContainer(ctx context.Context, r *kubeletdevicepluginv1beta1.PreStartContainerRequest) (*kubeletdevicepluginv1beta1.PreStartContainerResponse, error) {
	return &kubeletdevicepluginv1beta1.PreStartContainerResponse{}, nil
}

// GetPreferredAllocation returns a preferred set of devices to allocate
// from a list of available ones. The resulting preferred allocation is not
// guaranteed to be the allocation ultimately performed by the
// devicemanager. It is only designed to help the devicemanager make a more
// informed allocation decision when possible.
func (p *MockPlugin) GetPreferredAllocation(context.Context, *kubeletdevicepluginv1beta1.PreferredAllocationRequest) (*kubeletdevicepluginv1beta1.PreferredAllocationResponse, error) {
	return &kubeletdevicepluginv1beta1.PreferredAllocationResponse{}, nil
}

// ListAndWatch returns a stream of List of Devices
// Whenever a Device state change or a Device disappears, ListAndWatch
// returns the new list
func (p *MockPlugin) ListAndWatch(e *kubeletdevicepluginv1beta1.Empty, s kubeletdevicepluginv1beta1.DevicePlugin_ListAndWatchServer) error {
	devs := make([]*kubeletdevicepluginv1beta1.Device, p.Count)
	i := 0
	for {
		if i >= p.Count {
			break
		}
		dev := &kubeletdevicepluginv1beta1.Device{
			ID:     fmt.Sprintf("mock-devices-id-%d", i),
			Health: kubeletdevicepluginv1beta1.Healthy,
		}
		devs[i] = dev
		i++
	}
	klog.Infoln("Device Registered", p.ManagedResource, p.Count)
	s.Send(&kubeletdevicepluginv1beta1.ListAndWatchResponse{Devices: devs})
	for {
		time.Sleep(time.Second * 10)
		s.Send(&kubeletdevicepluginv1beta1.ListAndWatchResponse{Devices: devs})
	}
}

func (p *MockPlugin) Allocate(ctx context.Context, reqs *kubeletdevicepluginv1beta1.AllocateRequest) (*kubeletdevicepluginv1beta1.AllocateResponse, error) {
	var response kubeletdevicepluginv1beta1.AllocateResponse
	var car kubeletdevicepluginv1beta1.ContainerAllocateResponse

	klog.Infoln("Into Allocate")
	for range reqs.ContainerRequests {
		car = kubeletdevicepluginv1beta1.ContainerAllocateResponse{}
		response.ContainerResponses = append(response.ContainerResponses, &car)
	}

	return &response, nil
}

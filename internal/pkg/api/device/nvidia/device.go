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

package nvidia

import (
	"errors"
	"flag"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/HAMi/mock-device-plugin/internal/pkg/util"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	HandshakeAnnos      = "hami.io/node-handshake"
	RegisterAnnos       = "hami.io/node-nvidia-register"
	NvidiaGPUDevice     = "NVIDIA"
	NvidiaGPUCommonWord = "GPU"
	GPUInUse            = "nvidia.com/use-gputype"
	GPUNoUse            = "nvidia.com/nouse-gputype"
	NumaBind            = "nvidia.com/numa-bind"
	NodeLockNvidia      = "hami.io/mutex.lock"
	// GPUUseUUID is user can use specify GPU device for set GPU UUID.
	GPUUseUUID = "nvidia.com/use-gpuuuid"
	// GPUNoUseUUID is user can not use specify GPU device for set GPU UUID.
	GPUNoUseUUID = "nvidia.com/nouse-gpuuuid"
)

var (
	ResourceMem           string
	ResourceCores         string
	ResourceMemPercentage string
	Counts                = map[string]int{}
)

type NvidiaGPUDevices struct {
	DM *dpm.Manager
}

func InitNvidiaDevice(n *v1.Node) *NvidiaGPUDevices {
	_, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return nil
	}
	dev := &NvidiaGPUDevices{}
	devInfo, err := dev.GetNodeDevices(*n)
	if err != nil {
		klog.Infoln("decode annos err=", err.Error())
		return nil
	}
	index := strings.Index(ResourceMem, "/")
	for _, val := range devInfo {
		mock.Counts[ResourceMemPercentage[index+1:]] += 100
		mock.Counts[ResourceMem[index+1:]] += int(val.Devmem)
		mock.Counts[ResourceCores[index+1:]] += int(val.Devcore)
	}
	return dev
}

func (dev *NvidiaGPUDevices) RunManager() {
	klog.Infoln("runManager.....")
	index := strings.Index(ResourceMem, "/")
	lmock := mock.MockLister{
		ResUpdateChan: make(chan dpm.PluginNameList),
		Heartbeat:     make(chan bool),
		Namespace:     ResourceMem[:index],
	}
	mockmanager := dpm.NewManager(&lmock)

	go func() {
		lmock.ResUpdateChan <- []string{ResourceMem[index+1:], ResourceMemPercentage[index+1:], ResourceCores[index+1:]}
	}()
	klog.Infoln("Running mocking dp:nvidia")
	mockmanager.Run()
}

func ParseConfig() {
	flag.StringVar(&ResourceMem, "resource-mem", "nvidia.com/gpumem", "gpu memory to allocate")
	flag.StringVar(&ResourceMemPercentage, "resource-mem-percentage", "nvidia.com/gpumem-percentage", "gpu memory fraction to allocate")
	flag.StringVar(&ResourceCores, "resource-cores", "nvidia.com/gpucores", "cores percentage to use")
}

func (dev *NvidiaGPUDevices) GetNodeDevices(n v1.Node) ([]*api.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*api.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodedevices, err := util.DecodeNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*api.DeviceInfo{}, err
	}
	if len(nodedevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*api.DeviceInfo{}, errors.New("no gpu found on node")
	}
	devDecoded := util.EncodeNodeDevices(nodedevices)
	klog.V(5).InfoS("nodes device information", "node", n.Name, "nodedevices", devDecoded)
	return nodedevices, nil
}

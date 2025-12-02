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

type LibCudaLogLevel string
type GPUCoreUtilizationPolicy string

type NvidiaConfig struct {
	// These configs are shared and can be overwritten by Nodeconfig.
	NodeDefaultConfig            `yaml:",inline"`
	ResourceCountName            string `yaml:"resourceCountName"`
	ResourceMemoryName           string `yaml:"resourceMemoryName"`
	ResourceCoreName             string `yaml:"resourceCoreName"`
	ResourceMemoryPercentageName string `yaml:"resourceMemoryPercentageName"`
	ResourcePriority             string `yaml:"resourcePriorityName"`
	OverwriteEnv                 bool   `yaml:"overwriteEnv"`
	DefaultMemory                int32  `yaml:"defaultMemory"`
	DefaultCores                 int32  `yaml:"defaultCores"`
	DefaultGPUNum                int32  `yaml:"defaultGPUNum"`
	// TODO Whether these should be removed
	DisableCoreLimit  bool                          `yaml:"disableCoreLimit"`
	MigGeometriesList []AllowedMigGeometries `yaml:"knownMigGeometries"`
	// GPUCorePolicy through webhook automatic injected to container env
	GPUCorePolicy GPUCoreUtilizationPolicy `yaml:"gpuCorePolicy"`
	// RuntimeClassName is the name of the runtime class to be added to pod.spec.runtimeClassName
	RuntimeClassName string `yaml:"runtimeClassName"`
}

type MigTemplate struct {
	Name   string `yaml:"name"`
	Memory int32  `yaml:"memory"`
	Count  int32  `yaml:"count"`
}

type MigTemplateUsage struct {
	Name   string `json:"name,omitempty"`
	Memory int32  `json:"memory,omitempty"`
	InUse  bool   `json:"inuse,omitempty"`
}

type Geometry []MigTemplate

type AllowedMigGeometries struct {
	Models     []string   `yaml:"models"`
	Geometries []Geometry `yaml:"allowedGeometries"`
}

// These configs can be specified for each node by using Nodeconfig.
type NodeDefaultConfig struct {
	DeviceSplitCount    *uint    `yaml:"deviceSplitCount" json:"devicesplitcount"`
	DeviceMemoryScaling *float64 `yaml:"deviceMemoryScaling" json:"devicememoryscaling"`
	DeviceCoreScaling   *float64 `yaml:"deviceCoreScaling" json:"devicecorescaling"`
	// LogLevel is LIBCUDA_LOG_LEVEL value
	LogLevel *LibCudaLogLevel `yaml:"libCudaLogLevel" json:"libcudaloglevel"`
}

type NvidiaGPUDevices struct {
	config         NvidiaConfig
	ReportedGPUNum int64
}

func InitNvidiaDevice(nvconfig NvidiaConfig) *NvidiaGPUDevices {
	klog.InfoS("initializing nvidia device", "resourceName", nvconfig.ResourceCountName, "resourceMem", nvconfig.ResourceMemoryName, "DefaultGPUNum", nvconfig.DefaultGPUNum)
	return &NvidiaGPUDevices{
		config:         nvconfig,
		ReportedGPUNum: 0,
	}
}

func (dev *NvidiaGPUDevices) CommonWord() string {
	return NvidiaGPUDevice
}

func (dev *NvidiaGPUDevices) GetNodeDevices(n corev1.Node) ([]*device.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodedevices, err := device.UnMarshalNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, err
	}
	if len(nodedevices) == 0 {
		klog.InfoS("no nvidia gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, errors.New("no gpu found on node")
	}
	for idx := range nodedevices {
		nodedevices[idx].DeviceVendor = dev.CommonWord()
	}
	for _, val := range nodedevices {
		if val.Mode == MigMode {
			val.MIGTemplate = make([]device.Geometry, 0)
			for _, migTemplates := range dev.config.MigGeometriesList {
				found := false
				for _, migDevices := range migTemplates.Models {
					if strings.Contains(val.Type, migDevices) {
						found = true
						break
					}
				}
				if found {
					val.MIGTemplate = append(val.MIGTemplate, migTemplates.Geometries...)
					break
				}
			}
		}
	}

	pairScores, ok := n.Annotations[RegisterGPUPairScore]
	if !ok {
		klog.V(5).InfoS("no topology score found", "node", n.Name)
	} else {
		devicePairScores, err := device.DecodePairScores(pairScores)
		if err != nil {
			klog.ErrorS(err, "failed to decode pair scores", "node", n.Name, "pair scores", pairScores)
			return []*device.DeviceInfo{}, err
		}
		if devicePairScores != nil {
			// fit pair score to device info
			for _, deviceInfo := range nodedevices {
				uuid := deviceInfo.ID

				for _, devicePairScore := range *devicePairScores {
					if devicePairScore.ID == uuid {
						deviceInfo.DevicePairScore = devicePairScore
						break
					}
				}
			}
		}
	}
	devDecoded := device.EncodeNodeDevices(nodedevices)
	klog.V(5).InfoS("nodes device information", "node", n.Name, "nodedevices", devDecoded)
	return nodedevices, nil
}

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

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
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	RegisterAnnos        = "hami.io/node-nvidia-register"
	RegisterGPUPairScore = "hami.io/node-nvidia-score"
	NvidiaGPUDevice      = "NVIDIA"
	NvidiaGPUCommonWord  = "GPU"
	Vendor               = "nvidia.com"
	MigMode              = "mig"
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
	DisableCoreLimit  bool                   `yaml:"disableCoreLimit"`
	MigGeometriesList []AllowedMigGeometries `yaml:"knownMigGeometries"`
	// GPUCorePolicy through webhook automatic injected to container env
	GPUCorePolicy GPUCoreUtilizationPolicy `yaml:"gpuCorePolicy"`
	// RuntimeClassName is the name of the runtime class to be added to pod.spec.runtimeClassName
	RuntimeClassName string `yaml:"runtimeClassName"`
}

type AllowedMigGeometries struct {
	Models     []string          `yaml:"models"`
	Geometries []device.Geometry `yaml:"allowedGeometries"`
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
	lmock          mock.MockLister
}

func InitNvidiaDevice(nvconfig NvidiaConfig) *NvidiaGPUDevices {
	klog.InfoS("initializing nvidia device", "resourceName", nvconfig.ResourceCountName, "resourceMem", nvconfig.ResourceMemoryName, "DefaultGPUNum", nvconfig.DefaultGPUNum)
	return &NvidiaGPUDevices{
		config:         nvconfig,
		ReportedGPUNum: 0,
		lmock: mock.MockLister{
			ResUpdateChan: make(chan dpm.PluginNameList),
			Heartbeat:     make(chan bool),
			Namespace:     Vendor,
		},
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

func (dev *NvidiaGPUDevices) AddResource(n corev1.Node) {
	devs, err := dev.GetNodeDevices(n)
	if err != nil {
		klog.Warning("GetNodeDevices error:", err.Error())
		return
	}
	memoryResourceName := device.GetResourceName(dev.config.ResourceMemoryName)
	coreResourceName := device.GetResourceName(dev.config.ResourceCoreName)
	memoryPercentageName := device.GetResourceName(dev.config.ResourceMemoryPercentageName)
	for _, val := range devs {
		mock.Counts[memoryResourceName] += int(val.Devmem)
		mock.Counts[coreResourceName] += int(val.Devcore)
		mock.Counts[memoryPercentageName] += 100
	}
	klog.InfoS("Add resources",
				memoryResourceName,
				mock.Counts[memoryResourceName],
				coreResourceName,
				mock.Counts[coreResourceName],
				memoryPercentageName,
				mock.Counts[memoryPercentageName],
			)
	go func() {
		dev.lmock.ResUpdateChan <- []string{
			memoryResourceName,
			coreResourceName,
			memoryPercentageName,
		}
	}()
}

func (dev *NvidiaGPUDevices) RunManager() {
	mockmanager := dpm.NewManager(&dev.lmock)
	klog.Infoln("Running mocking dp: nvidia")
	mockmanager.Run()
}
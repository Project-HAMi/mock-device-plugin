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

package config

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/amd"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/ascend"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/awsneuron"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/cambricon"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/enflame"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/hygon"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/iluvatar"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/kunlun"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/metax"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/mthreads"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/nvidia"
)

type Config struct {
	NvidiaConfig    nvidia.NvidiaConfig       `yaml:"nvidia"`
	MetaxConfig     metax.MetaxConfig         `yaml:"metax"`
	HygonConfig     hygon.HygonConfig         `yaml:"hygon"`
	CambriconConfig cambricon.CambriconConfig `yaml:"cambricon"`
	MthreadsConfig  mthreads.MthreadsConfig   `yaml:"mthreads"`
	IluvatarConfig  []iluvatar.IluvatarConfig `yaml:"iluvatars"`
	EnflameConfig   enflame.EnflameConfig     `yaml:"enflame"`
	KunlunConfig    kunlun.KunlunConfig       `yaml:"kunlun"`
	AWSNeuronConfig awsneuron.AWSNeuronConfig `yaml:"awsneuron"`
	AMDGPUConfig    amd.AMDConfig             `yaml:"amd"`
	VNPUs           ascend.VNPUs              `yaml:"vnpus"`
}

var (
	configFile string
)

func LoadConfig(path string) (*Config, error) {
	klog.Infof("Reading config file from path: %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var yamlData Config
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return nil, err
	}
	klog.Info("Successfully read and parsed config file")
	return &yamlData, nil
}

func InitDevicesWithConfig(config *Config) error {
	iluvatarDevices, err := iluvatar.InitIluvatarDevices(config.IluvatarConfig)
	if err != nil {
		return err
	}
	if err := validateIluvatarCommonWordCollisions(config, iluvatarDevices); err != nil {
		return err
	}
	if err := validateIluvatarResourceCollisions(config); err != nil {
		return err
	}

	devicesMap := make(map[string]device.Devices)
	/*amdDevice := amd.InitAMDDevice(config.AMDGPUConfig)
	if amdDevice != nil {
		devicesMap[amdDevice.CommonWord()] = amdDevice
	}*/
	for _, dev := range ascend.InitDevices(config.VNPUs) {
		commonWord := dev.CommonWord()
		devicesMap[commonWord] = dev
		klog.Infof("Ascend device %s initialized", commonWord)
	}
	/*awsNeuronDevice := awsneuron.InitAWSNeuronDevice(config.AWSNeuronConfig)
	if awsNeuronDevice != nil {
		devicesMap[awsNeuronDevice.CommonWord()] = awsNeuronDevice
	}
	cambriconDevice := cambricon.InitMLUDevice(config.CambriconConfig)
	if cambriconDevice != nil {
		devicesMap[cambriconDevice.CommonWord()] = cambriconDevice
	}
	enflameDevice := enflame.InitEnflameVGCUDevice(config.EnflameConfig)
	if enflameDevice != nil {
		devicesMap[enflameDevice.CommonWord()] = enflameDevice
	}
	kunlunDevice := kunlun.InitKunlunVDevice(config.KunlunConfig)
	if kunlunDevice != nil {
		devicesMap[kunlunDevice.CommonWord()] = kunlunDevice
	}*/
	hygonDevice := hygon.InitDCUDevice(config.HygonConfig)
	if hygonDevice != nil {
		devicesMap[hygonDevice.CommonWord()] = hygonDevice
	}
	nvidiaDevice := nvidia.InitNvidiaDevice(config.NvidiaConfig)
	if nvidiaDevice != nil {
		devicesMap[nvidiaDevice.CommonWord()] = nvidiaDevice
	}
	for _, dev := range iluvatarDevices {
		devicesMap[dev.CommonWord()] = dev
		klog.Infof("Iluvatar device %s initialized", dev.CommonWord())
	}
	device.DevicesMap = devicesMap
	return nil
}

func validateIluvatarCommonWordCollisions(config *Config, iluvatarDevices []*iluvatar.IluvatarDevices) error {
	reserved := map[string]string{
		hygon.HygonDCUCommonWord: "Hygon",
		nvidia.NvidiaGPUDevice:   "NVIDIA",
	}
	for _, vnpuConfig := range config.VNPUs.Configs {
		reserved[vnpuConfig.CommonWord] = "Ascend " + vnpuConfig.CommonWord
	}
	for _, dev := range iluvatarDevices {
		if owner, exists := reserved[dev.CommonWord()]; exists {
			return fmt.Errorf("iluvatar commonWord %q conflicts with %s", dev.CommonWord(), owner)
		}
	}
	return nil
}

func validateIluvatarResourceCollisions(config *Config) error {
	reserved := make(map[string]string)
	add := func(owner, name string) {
		if name != "" {
			reserved[name] = owner
		}
	}

	add("NVIDIA", config.NvidiaConfig.ResourceCountName)
	add("NVIDIA", config.NvidiaConfig.ResourceMemoryName)
	add("NVIDIA", config.NvidiaConfig.ResourceCoreName)
	add("NVIDIA", config.NvidiaConfig.ResourceMemoryPercentageName)
	add("Hygon", config.HygonConfig.ResourceCountName)
	add("Hygon", config.HygonConfig.ResourceMemoryName)
	add("Hygon", config.HygonConfig.ResourceCoreName)
	for _, vnpuConfig := range config.VNPUs.Configs {
		add("Ascend "+vnpuConfig.CommonWord, vnpuConfig.ResourceName)
		add("Ascend "+vnpuConfig.CommonWord, vnpuConfig.ResourceMemoryName)
		add("Ascend "+vnpuConfig.CommonWord, vnpuConfig.ResourceCoreName)
	}

	for idx, iluvatarConfig := range config.IluvatarConfig {
		for _, name := range []string{iluvatarConfig.ResourceCountName, iluvatarConfig.ResourceMemoryName, iluvatarConfig.ResourceCoreName} {
			if owner, exists := reserved[name]; exists {
				return fmt.Errorf("iluvatar config at index %d reuses resource name %q from %s", idx, name, owner)
			}
		}
	}
	return nil
}

func InitDevices() {
	if len(device.DevicesMap) > 0 {
		klog.Info("Devices are already initialized, skipping initialization")
		return
	}
	klog.Infof("Loading device configuration from file: %s", configFile)
	config, err := LoadConfig(configFile)
	if err != nil {
		klog.Fatalf("Failed to load device config file %s: %v", configFile, err)
	}
	klog.Infof("Loaded config: %v", config)
	err = InitDevicesWithConfig(config)
	if err != nil {
		klog.Fatalf("Failed to initialize devices: %v", err)
	}
}

func GlobalFlagSet() {
	flag.StringVar(&configFile, "device-config-file", "", "Path to the device config file")
}

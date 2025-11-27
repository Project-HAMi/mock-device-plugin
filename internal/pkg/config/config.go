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
	"os"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

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
	VNPUs           []ascend.VNPUConfig       `yaml:"vnpus"`
}

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
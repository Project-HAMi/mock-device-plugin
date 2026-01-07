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

package device

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/HAMi/mock-device-plugin/internal/pkg/util/client"

	"github.com/ccoveille/go-safecast"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type DeviceInfo struct {
	ID              string          `json:"id,omitempty"`
	Index           uint            `json:"index,omitempty"`
	Count           int32           `json:"count,omitempty"`
	Devmem          int32           `json:"devmem,omitempty"`
	Devcore         int32           `json:"devcore,omitempty"`
	Type            string          `json:"type,omitempty"`
	Numa            int             `json:"numa,omitempty"`
	Mode            string          `json:"mode,omitempty"`
	MIGTemplate     []Geometry      `json:"migtemplate,omitempty"`
	Health          bool            `json:"health,omitempty"`
	DeviceVendor    string          `json:"devicevendor,omitempty"`
	CustomInfo      map[string]any  `json:"custominfo,omitempty"`
	DevicePairScore DevicePairScore `json:"devicepairscore,omitempty"`
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

type DevicePairScores []DevicePairScore
type DevicePairScore struct {
	ID     string         `json:"uuid,omitempty"`
	Scores map[string]int `json:"score,omitempty"`
}

type Devices interface {
	CommonWord() string
	GetNodeDevices(n *corev1.Node) ([]*DeviceInfo, error)
	GetResource(n *corev1.Node) map[string]int
	RunManager()
}

type ResourceNames struct {
	ResourceCountName  string
	ResourceMemoryName string
	ResourceCoreName   string
}

const (
	// OneContainerMultiDeviceSplitSymbol this is when one container use multi device, use : symbol to join device info.
	OneContainerMultiDeviceSplitSymbol = ":"

	// OnePodMultiContainerSplitSymbol this is when one pod having multi container and more than one container use device, use ; symbol to join device info.
	OnePodMultiContainerSplitSymbol = ";"
)

var (
	DevicesMap map[string]Devices
	ch         = map[string]chan int{}
)

func GetDevices() map[string]Devices {
	return DevicesMap
}

func RunManagers() error {
	for name, dev := range DevicesMap {
		klog.Infof("%s run manager", name)
		ch[name] = make(chan int)
		go dev.RunManager()
	}
	for _, val := range ch {
		<-val
	}
	return nil
}

func Register(l *mock.MockLister, dev Devices) {
	nodeName := os.Getenv("NODE_NAME")
	for {
		node, err := client.GetClient().CoreV1().Nodes().Get(context.Background(), nodeName, v1.GetOptions{})
		if err != nil {
			klog.Error("Get node error", err.Error())
		} else {
			resourceMap := dev.GetResource(node)
			l.SetResource(resourceMap)
		}
		time.Sleep(time.Second * 30)
	}
}

func GetResourceName(name string) string {
	if _, after, found := strings.Cut(name, "/"); found {
		return after
	}
	return name
}

func GetVendorName(name string) string {
	if before, _, found := strings.Cut(name, "/"); found {
		return before
	}
	return name
}

func UnMarshalNodeDevices(str string) ([]*DeviceInfo, error) {
	var dlist []*DeviceInfo
	err := json.Unmarshal([]byte(str), &dlist)
	return dlist, err
}

func DecodeNodeDevices(str string) ([]*DeviceInfo, error) {
	if !strings.Contains(str, OneContainerMultiDeviceSplitSymbol) {
		return []*DeviceInfo{}, errors.New("node annotations not decode successfully")
	}
	tmp := strings.Split(str, OneContainerMultiDeviceSplitSymbol)
	var retval []*DeviceInfo
	for _, val := range tmp {
		if strings.Contains(val, ",") {
			items := strings.Split(val, ",")
			if len(items) == 7 || len(items) == 9 {
				count, _ := strconv.ParseInt(items[1], 10, 32)
				devmem, _ := strconv.ParseInt(items[2], 10, 32)
				devcore, _ := strconv.ParseInt(items[3], 10, 32)
				health, _ := strconv.ParseBool(items[6])
				numa, _ := strconv.Atoi(items[5])
				mode := "hami-core"
				index := 0
				if len(items) == 9 {
					index, _ = strconv.Atoi(items[7])
					mode = items[8]
				}
				count32, err := safecast.Convert[int32](count)
				if err != nil {
					return []*DeviceInfo{}, errors.New("node annotations not decode successfully")
				}
				devmem32, err := safecast.Convert[int32](devmem)
				if err != nil {
					return []*DeviceInfo{}, errors.New("node annotations not decode successfully")
				}
				devcore32, err := safecast.Convert[int32](devcore)
				if err != nil {
					return []*DeviceInfo{}, errors.New("node annotations not decode successfully")
				}
				i := DeviceInfo{
					ID:      items[0],
					Count:   count32,
					Devmem:  devmem32,
					Devcore: devcore32,
					Type:    items[4],
					Numa:    numa,
					Health:  health,
					Mode:    mode,
					Index:   uint(index),
				}
				retval = append(retval, &i)
			} else {
				return []*DeviceInfo{}, errors.New("node annotations not decode successfully")
			}
		}
	}
	return retval, nil
}

func EncodeNodeDevices(dlist []*DeviceInfo) string {
	builder := strings.Builder{}
	for _, val := range dlist {
		builder.WriteString(val.ID)
		builder.WriteString(",")
		builder.WriteString(strconv.FormatInt(int64(val.Count), 10))
		builder.WriteString(",")
		builder.WriteString(strconv.Itoa(int(val.Devmem)))
		builder.WriteString(",")
		builder.WriteString(strconv.Itoa(int(val.Devcore)))
		builder.WriteString(",")
		builder.WriteString(val.Type)
		builder.WriteString(",")
		builder.WriteString(strconv.Itoa(val.Numa))
		builder.WriteString(",")
		builder.WriteString(strconv.FormatBool(val.Health))
		builder.WriteString(",")
		builder.WriteString(strconv.Itoa(int(val.Index)))
		builder.WriteString(",")
		builder.WriteString(val.Mode)
		builder.WriteString(OneContainerMultiDeviceSplitSymbol)
		//tmp += val.ID + "," + strconv.FormatInt(int64(val.Count), 10) + "," + strconv.Itoa(int(val.Devmem)) + "," + strconv.Itoa(int(val.Devcore)) + "," + val.Type + "," + strconv.Itoa(val.Numa) + "," + strconv.FormatBool(val.Health) + "," + strconv.Itoa(val.Index) + OneContainerMultiDeviceSplitSymbol
	}
	tmp := builder.String()
	klog.V(5).Infof("Encoded node Devices: %s", tmp)
	return tmp
}

func DecodePairScores(pairScores string) (*DevicePairScores, error) {
	devicePairScores := &DevicePairScores{}
	if err := json.Unmarshal([]byte(pairScores), devicePairScores); err != nil {
		return nil, err
	}
	return devicePairScores, nil
}

func CheckHealthy(n *corev1.Node, cardResourceName string) bool {
	capacity, exists := n.Status.Capacity[corev1.ResourceName(cardResourceName)]
	if !exists {
		return false
	}
	return !capacity.IsZero()
}

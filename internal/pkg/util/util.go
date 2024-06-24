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

package util

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api"
	"github.com/HAMi/mock-device-plugin/internal/pkg/util/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	// OneContainerMultiDeviceSplitSymbol this is when one container use multi device, use : symbol to join device info.
	OneContainerMultiDeviceSplitSymbol = ":"

	// OnePodMultiContainerSplitSymbol this is when one pod having multi container and more than one container use device, use ; symbol to join device info.
	OnePodMultiContainerSplitSymbol = ";"

	HandshakeAnnosString = "hami.io/node-handshake-dcu"
	RegisterAnnos        = "hami.io/node-dcu-register"
	HygonDCUDevice       = "DCU"
	HygonDCUCommonWord   = "DCU"
	DCUInUse             = "hygon.com/use-dcutype"
	DCUNoUse             = "hygon.com/nouse-dcutype"
	// DCUUseUUID is user can use specify DCU device for set DCU UUID.
	DCUUseUUID = "hygon.com/use-gpuuuid"
	// DCUNoUseUUID is user can not use specify DCU device for set DCU UUID.
	DCUNoUseUUID = "hygon.com/nouse-gpuuuid"
)

var (
	InRequestDevices map[string]string
	SupportDevices   map[string]string
	HandshakeAnnos   map[string]string
)

func init() {
	InRequestDevices = make(map[string]string)
	SupportDevices = make(map[string]string)
	HandshakeAnnos = make(map[string]string)
	InRequestDevices[HygonDCUDevice] = "hami.io/dcu-devices-to-allocate"
	SupportDevices[HygonDCUDevice] = "hami.io/dcu-devices-allocated"
	HandshakeAnnos[HygonDCUDevice] = HandshakeAnnosString
}

func GetNode(nodename string) (*corev1.Node, error) {
	n, err := client.GetClient().CoreV1().Nodes().Get(context.Background(), nodename, metav1.GetOptions{})
	return n, err
}

func GetPendingPod(node string) (*corev1.Pod, error) {
	podlist, err := client.GetClient().CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, p := range podlist.Items {
		if _, ok := p.Annotations[BindTimeAnnotations]; !ok {
			continue
		}
		if phase, ok := p.Annotations[DeviceBindPhase]; !ok {
			continue
		} else {
			if strings.Compare(phase, DeviceBindAllocating) != 0 {
				continue
			}
		}
		if n, ok := p.Annotations[AssignedNodeAnnotations]; !ok {
			continue
		} else {
			if strings.Compare(n, node) == 0 {
				return &p, nil
			}
		}
	}
	return nil, fmt.Errorf("no binding pod found on node %s", node)
}

func DecodeNodeDevices(str string) ([]*api.DeviceInfo, error) {
	if !strings.Contains(str, OneContainerMultiDeviceSplitSymbol) {
		return []*api.DeviceInfo{}, errors.New("node annotations not decode successfully")
	}
	tmp := strings.Split(str, OneContainerMultiDeviceSplitSymbol)
	var retval []*api.DeviceInfo
	for _, val := range tmp {
		if strings.Contains(val, ",") {
			items := strings.Split(val, ",")
			if len(items) == 7 {
				count, _ := strconv.Atoi(items[1])
				devmem, _ := strconv.Atoi(items[2])
				devcore, _ := strconv.Atoi(items[3])
				health, _ := strconv.ParseBool(items[6])
				numa, _ := strconv.Atoi(items[5])
				i := api.DeviceInfo{
					Id:      items[0],
					Count:   int32(count),
					Devmem:  int32(devmem),
					Devcore: int32(devcore),
					Type:    items[4],
					Numa:    numa,
					Health:  health,
				}
				retval = append(retval, &i)
			} else {
				return []*api.DeviceInfo{}, errors.New("node annotations not decode successfully")
			}
		}
	}
	return retval, nil
}

func EncodeNodeDevices(dlist []*api.DeviceInfo) string {
	tmp := ""
	for _, val := range dlist {
		tmp += val.Id + "," + strconv.FormatInt(int64(val.Count), 10) + "," + strconv.Itoa(int(val.Devmem)) + "," + strconv.Itoa(int(val.Devcore)) + "," + val.Type + "," + strconv.Itoa(val.Numa) + "," + strconv.FormatBool(val.Health) + OneContainerMultiDeviceSplitSymbol
	}
	klog.Infof("Encoded node Devices: %s", tmp)
	return tmp
}

func EncodeContainerDevices(cd ContainerDevices) string {
	tmp := ""
	for _, val := range cd {
		tmp += val.UUID + "," + val.Type + "," + strconv.Itoa(int(val.Usedmem)) + "," + strconv.Itoa(int(val.Usedcores)) + OneContainerMultiDeviceSplitSymbol
	}
	klog.Infof("Encoded container Devices: %s", tmp)
	return tmp
	//return strings.Join(cd, ",")
}

func EncodeContainerDeviceType(cd ContainerDevices, t string) string {
	tmp := ""
	for _, val := range cd {
		if strings.Compare(val.Type, t) == 0 {
			tmp += val.UUID + "," + val.Type + "," + strconv.Itoa(int(val.Usedmem)) + "," + strconv.Itoa(int(val.Usedcores))
		}
		tmp += OneContainerMultiDeviceSplitSymbol
	}
	klog.Infof("Encoded container Certain Device type: %s->%s", t, tmp)
	return tmp
}

func EncodePodSingleDevice(pd PodSingleDevice) string {
	res := ""
	for _, ctrdevs := range pd {
		res = res + EncodeContainerDevices(ctrdevs)
		res = res + OnePodMultiContainerSplitSymbol
	}
	klog.Infof("Encoded pod single devices %s", res)
	return res
}

func EncodePodDevices(checklist map[string]string, pd PodDevices) map[string]string {
	res := map[string]string{}
	for devType, cd := range pd {
		klog.Infoln("devtype=", devType)
		res[checklist[devType]] = EncodePodSingleDevice(cd)
	}
	klog.Infof("Encoded pod Devices %s\n", res)
	return res
}

func DecodeContainerDevices(str string) (ContainerDevices, error) {
	if len(str) == 0 {
		return ContainerDevices{}, nil
	}
	cd := strings.Split(str, OneContainerMultiDeviceSplitSymbol)
	contdev := ContainerDevices{}
	tmpdev := ContainerDevice{}
	klog.V(5).Infof("Start to decode container device %s", str)
	if len(str) == 0 {
		return ContainerDevices{}, nil
	}
	for _, val := range cd {
		if strings.Contains(val, ",") {
			//fmt.Println("cd is ", val)
			tmpstr := strings.Split(val, ",")
			if len(tmpstr) < 4 {
				return ContainerDevices{}, fmt.Errorf("pod annotation format error; information missing, please do not use nodeName field in task")
			}
			tmpdev.UUID = tmpstr[0]
			tmpdev.Type = tmpstr[1]
			devmem, _ := strconv.ParseInt(tmpstr[2], 10, 32)
			tmpdev.Usedmem = int32(devmem)
			devcores, _ := strconv.ParseInt(tmpstr[3], 10, 32)
			tmpdev.Usedcores = int32(devcores)
			contdev = append(contdev, tmpdev)
		}
	}
	klog.V(5).Infof("Finished decoding container devices. Total devices: %d", len(contdev))
	return contdev, nil
}

func DecodePodDevices(checklist map[string]string, annos map[string]string) (PodDevices, error) {
	klog.V(5).Infof("checklist is [%+v], annos is [%+v]", checklist, annos)
	if len(annos) == 0 {
		return PodDevices{}, nil
	}
	pd := make(PodDevices)
	for devID, devs := range checklist {
		str, ok := annos[devs]
		if !ok {
			continue
		}
		pd[devID] = make(PodSingleDevice, 0)
		for _, s := range strings.Split(str, OnePodMultiContainerSplitSymbol) {
			cd, err := DecodeContainerDevices(s)
			if err != nil {
				return PodDevices{}, nil
			}
			if len(cd) == 0 {
				continue
			}
			pd[devID] = append(pd[devID], cd)
		}
	}
	klog.InfoS("Decoded pod annos", "poddevices", pd)
	return pd, nil
}

func GetNextDeviceRequest(dtype string, p corev1.Pod) (corev1.Container, ContainerDevices, error) {
	pdevices, err := DecodePodDevices(InRequestDevices, p.Annotations)
	if err != nil {
		return corev1.Container{}, ContainerDevices{}, err
	}
	klog.Infof("pod annotation decode vaule is %+v", pdevices)
	res := ContainerDevices{}

	pd, ok := pdevices[dtype]
	if !ok {
		return corev1.Container{}, res, errors.New("device request not found")
	}
	for ctridx, ctrDevice := range pd {
		if len(ctrDevice) > 0 {
			return p.Spec.Containers[ctridx], ctrDevice, nil
		}
	}
	return corev1.Container{}, res, errors.New("device request not found")
}

func GetContainerDeviceStrArray(c ContainerDevices) []string {
	tmp := []string{}
	for _, val := range c {
		tmp = append(tmp, val.UUID)
	}
	return tmp
}

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

package hygon

import (
	"testing"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDCUDevices_GetResource(t *testing.T) {

	config := HygonConfig{
		ResourceCountName:  "hygon.com/dcunum",
		ResourceMemoryName: "hygon.com/dcumem",
		ResourceCoreName:   "hygon.com/dcucores",
	}

	t.Run("WithValidAnnotation", func(t *testing.T) {
		dev := InitDCUDevice(config)

		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-1",
				Annotations: map[string]string{
					RegisterAnnos: "DCU-TR3A380008110601,4,65520,100,DCU-K100_AI,0,true,2,hami:DCU-TPYX300018090901,4,65520,100,DCU-K100_AI,0,true,3,hami:DCU-TPYX300009040801,4,65520,100,DCU-K100_AI,0,true,4,hami:DCU-TR3A380013080301,4,65520,100,DCU-K100_AI,0,true,5,hami:DCU-TPYX300003040301,4,65520,100,DCU-K100_AI,0,true,0,hami:DCU-TPYX360008060301,4,65520,100,DCU-K100_AI,0,true,1,hami:",
				},
			},
			Status: corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceName(config.ResourceCountName): resource.MustParse("4"),
				},
			},
		}

		result := dev.GetResource(node)
		resourceName := device.GetResourceName(config.ResourceMemoryName)

		expectedTotalMemory := 65520 * 6

		if result[resourceName] != expectedTotalMemory {
			t.Errorf("Expected total memory %d, got %d", expectedTotalMemory, result[resourceName])
		}

	})
}

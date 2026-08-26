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

package device

import (
	"encoding/json"
	"errors"
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DecodeNodeDevices(t *testing.T) {
	tests := []struct {
		name string
		args string
		want struct {
			di  []*DeviceInfo
			err error
		}
	}{
		{
			name: "args is invalid",
			args: "a",
			want: struct {
				di  []*DeviceInfo
				err error
			}{
				di:  []*DeviceInfo{},
				err: errors.New("node annotations not decode successfully"),
			},
		},
		{
			name: "str is old format",
			args: "GPU-ebe7c3f7-303d-558d-435e-99a160631fe4,10,7680,100,NVIDIA-Tesla P4,0,true:",
			want: struct {
				di  []*DeviceInfo
				err error
			}{
				di: []*DeviceInfo{
					{
						ID:      "GPU-ebe7c3f7-303d-558d-435e-99a160631fe4",
						Index:   0,
						Count:   10,
						Devmem:  7680,
						Devcore: 100,
						Type:    "NVIDIA-Tesla P4",
						Mode:    "hami-core",
						Numa:    0,
						Health:  true,
					},
				},
				err: nil,
			},
		},
		{
			name: "str is new format",
			args: "GPU-ebe7c3f7-303d-558d-435e-99a160631fe4,10,7680,100,NVIDIA-Tesla P4,0,true,1,hami-core:",
			want: struct {
				di  []*DeviceInfo
				err error
			}{
				di: []*DeviceInfo{
					{
						ID:      "GPU-ebe7c3f7-303d-558d-435e-99a160631fe4",
						Index:   1,
						Count:   10,
						Devmem:  7680,
						Devcore: 100,
						Type:    "NVIDIA-Tesla P4",
						Mode:    "hami-core",
						Numa:    0,
						Health:  true,
					},
				},
				err: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeNodeDevices(test.args)
			assert.DeepEqual(t, test.want.di, got)
			if err != nil {
				assert.DeepEqual(t, test.want.err.Error(), err.Error())
			}
		})
	}
}

func TestHandshakePatch(t *testing.T) {
	const annotation = "hami.io/node-handshake-xpu"
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "mock-node",
		Annotations: map[string]string{annotation: "Requesting_2026-08-26 03:00:00"},
	}}

	patch, ok, err := handshakePatch(node, annotation)
	assert.NilError(t, err)
	assert.Assert(t, ok)
	var operations []map[string]string
	err = json.Unmarshal(patch, &operations)
	assert.NilError(t, err)
	assert.Equal(t, len(operations), 2)
	assert.DeepEqual(t, operations[0], map[string]string{
		"op":    "test",
		"path":  "/metadata/annotations/hami.io~1node-handshake-xpu",
		"value": "Requesting_2026-08-26 03:00:00",
	})
	assert.DeepEqual(t, operations[1], map[string]string{
		"op":    "replace",
		"path":  "/metadata/annotations/hami.io~1node-handshake-xpu",
		"value": "Reported_2026-08-26 03:00:00",
	})
}

func TestHandshakePatchIgnoresNonRequest(t *testing.T) {
	const annotation = "hami.io/node-handshake-xpu"
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "mock-node",
		Annotations: map[string]string{annotation: "Reported_2026-08-26 03:00:00"},
	}}

	patch, ok, err := handshakePatch(node, annotation)
	assert.NilError(t, err)
	assert.Assert(t, !ok)
	assert.Assert(t, patch == nil)
}

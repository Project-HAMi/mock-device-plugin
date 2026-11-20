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
	"errors"
	"io"
	"testing"
	"time"

	dp "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type watchStream struct {
	dp.DevicePlugin_ListAndWatchServer
	ctx     context.Context
	sent    chan struct{}
	sendErr error
}

func (s *watchStream) Context() context.Context { return s.ctx }

func (s *watchStream) Send(*dp.ListAndWatchResponse) error {
	select {
	case s.sent <- struct{}{}:
	default:
	}
	return s.sendErr
}

func TestListAndWatchEndsOnDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sendErr, want error
	}{
		{"send error", io.ErrClosedPipe, io.ErrClosedPipe},
		{"context cancelled", nil, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream := &watchStream{ctx: ctx, sent: make(chan struct{}, 1), sendErr: tc.sendErr}
			done := make(chan error, 1)
			go func() { done <- new(MockPlugin).ListAndWatch(&dp.Empty{}, stream) }()
			select {
			case <-stream.sent:
			case <-time.After(time.Second):
				t.Fatal("no initial device update")
			}
			if tc.sendErr == nil {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v, want %v", err, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("watch handler still running")
			}
		})
	}
}

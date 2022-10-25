// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/wlan"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			wifi.RegisterDeviceServiceServer(srv, &DeviceService{})
		},
	})
}

type DeviceService struct{}

func (s *DeviceService) GetDeviceInfo(ctx context.Context, _ *empty.Empty) (*wifi.GetDeviceInfoResponse, error) {
	// Get the information of the WLAN device.
	devInfo, err := wlan.DeviceInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the WLAN device information")
	}
	return &wifi.GetDeviceInfoResponse{
		Name: devInfo.Name,
	}, nil

}

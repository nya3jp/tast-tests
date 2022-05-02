// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"io/ioutil"
	"strings"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	ts "chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ts.RegisterServiceServer(srv, &Service{})
		},
	})
}

// Service implements tast.cros.tape.Service.
type Service struct {
}

// GetDeviceID retrieves the device id from the /var/lib/devicesettings/policy.1 file.
func (service *Service) GetDeviceID(ctx context.Context, req *ts.GetDeviceIDRequest) (resp *ts.GetDeviceIDResponse, retErr error) {
	const deviceSettingsFileName = "/var/lib/devicesettings/policy.1"
	const deviceIDLength = 36

	data, err := ioutil.ReadFile(deviceSettingsFileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %s", deviceSettingsFileName)
	}

	deviceSettings := strings.ToValidUTF8(string(data), "")
	pos := strings.Index(deviceSettings, "\t"+req.CustomerID)
	deviceID := deviceSettings[pos-deviceIDLength-1 : pos-1]

	return &ts.GetDeviceIDResponse{DeviceID: deviceID}, nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// Helper fetches Cellular Device and Service properties.
type Helper struct {
	manager *shill.Manager
	ctx     context.Context
	Device  *shill.Device
	Service *shill.Service // The Cellular Service matching the Device ICCID.
}

// NewHelper creates a Helper object and ensures that a Cellular Device and
// corresponding Service (identified by ICCID) are present. It also ensures
// that the Service is Connectable.
func NewHelper(ctx context.Context) (*Helper, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	device, err := manager.DeviceByType(ctx, shillconst.TypeCellular)
	if err != nil || device == nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device")
	}
	deviceProperties, err := device.GetProperties(ctx)
	if err != nil || device == nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device properties")
	}
	deviceIccid, err := deviceProperties.GetString(shillconst.DevicePropertyCellularICCID)
	if deviceIccid == "" {
		return nil, errors.Wrap(err, "device missing ICCID")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: deviceIccid,
		shillconst.ServicePropertyConnectable:   true,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	service, err := manager.FindMatchingService(ctx, props)
	if err != nil || service == nil {
		return nil, errors.Wrap(err, "failed to find matching Cellular Service")
	}

	return &Helper{
		manager: manager,
		ctx:     ctx,
		Device:  device,
		Service: service,
	}, nil
}

// TODO(stevenjb): Use shorter sleep durations and query state in a loop.

// IsEnabled returns true if Cellular is enabled.
func (helper *Helper) IsEnabled() bool {
	enabled, _ := helper.manager.IsEnabled(helper.ctx, shill.TechnologyCellular)
	return enabled
}

// Enable attempts to enable Cellular returns true if enabled after the attempt.
func (helper *Helper) Enable() bool {
	helper.manager.EnableTechnology(helper.ctx, shill.TechnologyCellular)
	testing.Sleep(helper.ctx, 3*time.Second)
	return helper.IsEnabled()
}

// Disable attempts to disable Cellular returns true if disabled after the attempt.
func (helper *Helper) Disable() bool {
	helper.manager.DisableTechnology(helper.ctx, shill.TechnologyCellular)
	testing.Sleep(helper.ctx, 1*time.Second)
	return !helper.IsEnabled()
}

// IsConnected returns true if Cellular is connected.
func (helper *Helper) IsConnected() bool {
	properties, err := helper.Service.GetProperties(helper.ctx)
	if err != nil {
		return false
	}
	connected, err := properties.GetBool(shillconst.ServicePropertyIsConnected)
	return connected
}

// Connect attempts to connect to the Cellular Service and returns true if connected after the attempt.
func (helper *Helper) Connect() bool {
	helper.Service.Connect(helper.ctx)
	testing.Sleep(helper.ctx, 1*time.Second)
	return helper.IsConnected()
}

// Disconnect attempts to disconnect from the Cellular Service and returns true if disconnected after the attempt.
func (helper *Helper) Disconnect() bool {
	helper.Service.Disconnect(helper.ctx)
	testing.Sleep(helper.ctx, 1*time.Second)
	return !helper.IsConnected()
}

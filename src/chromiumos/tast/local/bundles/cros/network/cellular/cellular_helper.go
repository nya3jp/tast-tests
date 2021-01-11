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
	ctx     context.Context
	Manager *shill.Manager
	Device  *shill.Device
}

// NewHelper creates a Helper object and ensures that a Cellular Device is present.
func NewHelper(ctx context.Context) (*Helper, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	available, err := manager.IsAvailable(ctx, shill.TechnologyCellular)
	if err != nil || !available {
		return nil, errors.Wrap(err, "Cellular Technology not available")
	}
	device, err := manager.DeviceByType(ctx, shillconst.TypeCellular)
	if err != nil || device == nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device")
	}
	return &Helper{
		ctx:     ctx,
		Manager: manager,
		Device:  device,
	}, nil
}

// IsEnabled returns true if Cellular is enabled.
func (helper *Helper) IsEnabled() bool {
	enabled, _ := helper.Manager.IsEnabled(helper.ctx, shill.TechnologyCellular)
	return enabled
}

// Enable enables Cellular and ensures that the enable succeeded, otherwise an error is returned.
func (helper *Helper) Enable() error {
	helper.Manager.EnableTechnology(helper.ctx, shill.TechnologyCellular)

	// It may take a few seconds for Cellular to become enabled.
	if err := testing.Poll(helper.ctx, func(ctx context.Context) error {
		enabled, err := helper.Manager.IsEnabled(ctx, shill.TechnologyCellular)
		if err != nil {
			return err
		}
		if !enabled {
			return errors.New("Cellular not enabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 100 * time.Millisecond,
	}); err != nil {
		return err
	}
	return nil
}

func (helper *Helper) getMatchingService(props map[string]interface{}) (*shill.Service, error) {
	// It may take a few seconds for a Cellular Service to appear.
	var cellularService *shill.Service
	if err := testing.Poll(helper.ctx, func(ctx context.Context) error {
		cellularService, err := helper.Manager.FindMatchingService(ctx, props)
		if err != nil {
			return err
		}
		if cellularService == nil {
			return errors.New("Cellular Service not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 100 * time.Millisecond,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to get Cellular Service")
	}
	return cellularService, nil
}

// GetService returns the first connectable Cellular Service.
// If no such Cellular Service is available, returns a nil service and an error.
func (helper *Helper) GetService() (*shill.Service, error) {
	cellularProps := map[string]interface{}{
		shillconst.ServicePropertyConnectable: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	return helper.getMatchingService(cellularProps)
}

// GetServiceForDevice returns the first connectable Cellular Service matching the Device ICCID.
// If no such Cellular Service is available, returns a nil service and an error.
func (helper *Helper) GetServiceForDevice() (*shill.Service, error) {
	deviceProperties, err := helper.Device.GetProperties(helper.ctx)
	if err != nil || deviceProperties == nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device properties")
	}
	deviceIccid, err := deviceProperties.GetString(shillconst.DevicePropertyCellularICCID)
	if deviceIccid == "" {
		return nil, errors.Wrap(err, "device missing ICCID")
	}
	cellularProps := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: deviceIccid,
		shillconst.ServicePropertyConnectable:   true,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	return helper.getMatchingService(cellularProps)
}

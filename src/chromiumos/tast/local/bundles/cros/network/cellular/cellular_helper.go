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

// IsEnabled returns true if Cellular is enabled. If an error occurs, returns false.
func (h *Helper) IsEnabled() bool {
	enabled, _ := h.Manager.IsEnabled(h.ctx, shill.TechnologyCellular)
	return enabled
}

func (h *Helper) waitForEnabled(expected bool) error {
	return testing.Poll(h.ctx, func(ctx context.Context) error {
		enabled, err := h.Manager.IsEnabled(ctx, shill.TechnologyCellular)
		if err != nil {
			return errors.Wrap(err, "failed to get enabled state")
		}
		if enabled != expected {
			return errors.Errorf("enabled != %t", expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  3 * time.Second,
		Interval: 100 * time.Millisecond,
	})
}

func (h *Helper) waitForPowered(expected bool) error {
	return testing.Poll(h.ctx, func(ctx context.Context) error {
		deviceProperties, err := h.Device.GetProperties(h.ctx)
		if err != nil {
			return err
		}
		powered, err := deviceProperties.GetBool(shillconst.DevicePropertyPowered)
		if err != nil {
			return err
		}
		if powered != expected {
			return errors.Errorf("powered != %t", expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  3 * time.Second,
		Interval: 100 * time.Millisecond,
	})
}

// Enable Cellular and ensure that the enable succeeded, otherwise returns an error.
func (h *Helper) Enable() error {
	h.Manager.EnableTechnology(h.ctx, shill.TechnologyCellular)

	if err := h.waitForEnabled(true); err != nil {
		return err
	}
	return h.waitForPowered(true)
}

// Disable Cellular and ensure that the disable succeeded, otherwise return an error.
func (h *Helper) Disable() error {
	h.Manager.DisableTechnology(h.ctx, shill.TechnologyCellular)

	if err := h.waitForEnabled(false); err != nil {
		return err
	}
	err := h.waitForPowered(false)
	// Operations (i.e. Enable) called immediately after disabling can fail.
	// TODO(b/177588333): Fix instead of sleeping here.
	testing.Sleep(h.ctx, 200*time.Millisecond)
	return err
}

// FindService returns the first connectable Cellular Service.
func (h *Helper) FindService() (*shill.Service, error) {
	// Look for any connectable Cellular service.
	cellularProperties := map[string]interface{}{
		shillconst.ServicePropertyConnectable: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	return h.Manager.WaitForServiceProperties(h.ctx, cellularProperties, 5*time.Second)
}

// FindServiceForDevice returns the first connectable Cellular Service matching the Device ICCID.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindServiceForDevice() (*shill.Service, error) {
	deviceProperties, err := h.Device.GetProperties(h.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device properties")
	}
	deviceIccid, err := deviceProperties.GetString(shillconst.DevicePropertyCellularICCID)
	if err != nil || deviceIccid == "" {
		return nil, errors.Wrap(err, "device missing ICCID")
	}
	cellularProperties := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: deviceIccid,
		shillconst.ServicePropertyConnectable:   true,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	return h.Manager.WaitForServiceProperties(h.ctx, cellularProperties, 5*time.Second)
}

func (h *Helper) isConnected(service *shill.Service) (bool, error) {
	properties, err := service.GetProperties(h.ctx)
	if err != nil {
		return false, err
	}
	return properties.GetBool(shillconst.ServicePropertyIsConnected)
}

func (h *Helper) waitForConnected(service *shill.Service, expected bool) error {
	return testing.Poll(h.ctx, func(ctx context.Context) error {
		connected, err := h.isConnected(service)
		if err != nil {
			return err
		}
		if connected != expected {
			return errors.Errorf("Cellular Service connected != %t", expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  6 * time.Second,
		Interval: 100 * time.Millisecond,
	})
}

// Connect to the Cellular Service and ensure that the connect succeeded, otherwise return an error.
func (h *Helper) Connect() error {
	service, err := h.FindServiceForDevice()
	if err != nil {
		return err
	}
	if err := service.Connect(h.ctx); err != nil {
		return err
	}
	return h.waitForConnected(service, true)
}

// Disconnect from the Cellular Service and ensure that the disconnect succeeded, otherwise return an error.
func (h *Helper) Disconnect() error {
	service, err := h.FindServiceForDevice()
	if err != nil {
		return err
	}
	if err := service.Disconnect(h.ctx); err != nil {
		return err
	}
	return h.waitForConnected(service, false)
}

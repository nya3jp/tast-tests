// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
	helper := Helper{Manager: manager, Device: device}

	// Ensure Cellular is enabled.
	if enabled, err := manager.IsEnabled(ctx, shill.TechnologyCellular); err != nil {
		return nil, errors.Wrap(err, "error requesting enabled state")
	} else if !enabled {
		if err := helper.Enable(ctx); err != nil {
			return nil, errors.Wrap(err, "unable to enable Cellular")
		}
	}

	return &helper, nil
}

// waitForEnabled polls for the specified enable state for cellular.
func (h *Helper) waitForEnabled(ctx context.Context, expected bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		enabled, err := h.Manager.IsEnabled(ctx, shill.TechnologyCellular)
		if err != nil {
			return errors.Wrap(err, "failed to get enabled state")
		}
		if enabled != expected {
			return errors.Errorf("unexpected enabled state, got %t, expected %t", enabled, expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  3 * time.Second,
		Interval: 100 * time.Millisecond,
	})
}

// Enable calls Manager.EnableTechnology(cellular) and returns true if the enable succeeded, or an error otherwise.
func (h *Helper) Enable(ctx context.Context) error {
	h.Manager.EnableTechnology(ctx, shill.TechnologyCellular)

	if err := h.waitForEnabled(ctx, true); err != nil {
		return err
	}
	if err := h.Device.WaitForShillProperty(ctx, shillconst.DevicePropertyPowered, true, 3*time.Second); err != nil {
		return err
	}
	// Cellular scanning can take up to 30 seconds to complete.
	return h.Device.WaitForShillProperty(ctx, shillconst.DevicePropertyScanning, false, 30*time.Second)
}

// Disable calls Manager.DisableTechnology(cellular) and returns true if the disable succeeded, or an error otherwise.
func (h *Helper) Disable(ctx context.Context) error {
	h.Manager.DisableTechnology(ctx, shill.TechnologyCellular)

	if err := h.waitForEnabled(ctx, false); err != nil {
		return err
	}
	err := h.Device.WaitForShillProperty(ctx, shillconst.DevicePropertyPowered, false, 3*time.Second)
	// Operations (i.e. Enable) called immediately after disabling can fail.
	// TODO(b/177588333): Fix instead of sleeping here.
	testing.Sleep(ctx, 1000*time.Millisecond)
	return err
}

// FindService returns the first connectable Cellular Service.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindService(ctx context.Context) (*shill.Service, error) {
	// Look for any connectable Cellular service.
	cellularProperties := map[string]interface{}{
		shillconst.ServicePropertyConnectable: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	return h.Manager.WaitForServiceProperties(ctx, cellularProperties, 5*time.Second)
}

// FindServiceForDevice returns the first connectable Cellular Service matching the Device ICCID.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindServiceForDevice(ctx context.Context) (*shill.Service, error) {
	deviceProperties, err := h.Device.GetShillProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device properties")
	}
	deviceIccid, err := deviceProperties.GetString(shillconst.DevicePropertyCellularICCID)
	if err != nil {
		return nil, errors.Wrap(err, "device missing ICCID")
	}
	if deviceIccid == "" {
		return nil, errors.Wrap(err, "device has empty ICCID")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: deviceIccid,
		shillconst.ServicePropertyConnectable:   true,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	service, err := h.Manager.WaitForServiceProperties(ctx, props, 5*time.Second)
	if err != nil {
		return nil, errors.Wrapf(err, "Service not found for: %+v", props)
	}
	return service, nil
}

// AutoConnectCleanupTime provides enough time for a successful dbus operation.
// If a timeout occurs during cleanup, the operation will fail anyway.
const AutoConnectCleanupTime = 1 * time.Second

// SetServiceAutoConnect sets the AutoConnect property of the Cellular Service
// associated with the Cellular Device to |autoConnect| if the current value
// does not match |autoConnect|.
// Returns true when Service.AutoConnect is set and the operation succeeds.
// Returns an error if any operation fails.
func (h *Helper) SetServiceAutoConnect(ctx context.Context, autoConnect bool) (bool, error) {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get Cellular Service")
	}
	properties, err := service.GetShillProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get properties")
	}
	curAutoConnect, err := properties.GetBool(shillconst.ServicePropertyAutoConnect)
	if err != nil {
		return false, errors.Wrap(err, "unable to get AutoConnect")
	}
	if autoConnect == curAutoConnect {
		return false, nil
	}
	if err := service.SetProperty(ctx, shillconst.ServicePropertyAutoConnect, autoConnect); err != nil {
		return false, errors.Wrap(err, "failed to set Service.AutoConnect")
	}
	return true, nil
}

// Connect to the Cellular Service and ensure that the connect succeeded, otherwise return an error.
func (h *Helper) Connect(ctx context.Context) error {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return err
	}
	if err := service.Connect(ctx); err != nil {
		return err
	}
	return service.WaitForShillProperty(ctx, shillconst.ServicePropertyIsConnected, true, 6*time.Second)
}

// Disconnect from the Cellular Service and ensure that the disconnect succeeded, otherwise return an error.
func (h *Helper) Disconnect(ctx context.Context) error {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return err
	}
	if err := service.Disconnect(ctx); err != nil {
		return err
	}
	return service.WaitForShillProperty(ctx, shillconst.ServicePropertyIsConnected, false, 6*time.Second)
}

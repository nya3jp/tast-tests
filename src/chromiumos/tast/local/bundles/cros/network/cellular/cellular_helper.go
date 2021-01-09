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
	helper := Helper{ctx: ctx, Manager: manager, Device: device}

	// Ensure Cellular is enabled.
	if !helper.IsEnabled() {
		if err := helper.Enable(); err != nil {
			return nil, errors.Wrap(err, "unable to enable Cellular")
		}
	}

	return &helper, nil
}

// IsEnabled returns true if Cellular is enabled. If an error occurs, returns false.
func (h *Helper) IsEnabled() bool {
	enabled, _ := h.Manager.IsEnabled(h.ctx, shill.TechnologyCellular)
	return enabled
}

// Enable enables Cellular and ensures that the enable succeeded, otherwise an error is returned.
func (h *Helper) Enable() error {
	h.Manager.EnableTechnology(h.ctx, shill.TechnologyCellular)

	// It may take a few seconds for Cellular to become enabled.
	if err := testing.Poll(h.ctx, func(ctx context.Context) error {
		enabled, err := h.Manager.IsEnabled(ctx, shill.TechnologyCellular)
		if err != nil {
			return errors.Wrap(err, "failed to get enabled state")
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

// FindService returns the first connectable Cellular Service.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindService() (*shill.Service, error) {
	// Look for any connectable Cellular service.
	cellularProperties := map[string]interface{}{
		shillconst.ServicePropertyConnectable: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	return h.Manager.WaitForServiceProperties(h.ctx, cellularProperties, 5*time.Second)
}

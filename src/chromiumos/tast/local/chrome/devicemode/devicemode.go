// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package devicemode contains API for switching the device mode.
package devicemode

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
)

// DeviceMode represents whether the device is tablet or clamshell.
type DeviceMode int

// Valid device mode enums.
const (
	DefaultMode DeviceMode = iota
	TabletMode
	ClamshellMode
)

func (mode DeviceMode) String() string {
	switch mode {
	case DefaultMode:
		return "default"
	case TabletMode:
		return "tablet"
	case ClamshellMode:
		return "clamshell"
	}
	return "unknown value"
}

// EnsureDeviceMode makes sure that the given mode state is enabled,
// and returns a function which reverts back to the original state.
//
// Typically, this will be used like:
//
//	cleanup, err := ash.EnsureDeviceMode(ctx, c, chrome.TabletMode)
//	if err != nil {
//	  s.Fatal("Failed to ensure in tablet mode: ", err)
//	}
//	defer cleanup(ctx)
func EnsureDeviceMode(ctx context.Context, tconn *chrome.TestConn, mode DeviceMode) (func(ctx context.Context) error, error) {
	switch mode {
	case DefaultMode:
		return func(ctx context.Context) error { return nil }, nil
	case TabletMode:
		return ash.EnsureTabletModeEnabled(ctx, tconn, true)
	case ClamshellMode:
		return ash.EnsureTabletModeEnabled(ctx, tconn, false)
	}
	return nil, errors.Errorf("unsupported device mode: %s(%d)", mode.String(), mode)
}

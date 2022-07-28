// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rgbkbd contains utilities for communicating with rgbkbd.
package rgbkbd

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.Rgbkbd"
	dbusPath      = "/org/chromium/Rgbkbd"
	dbusInterface = "org.chromium.Rgbkbd"
)

// Rgbkbd is used to interact with the rgbkbd process over D-Bus.
type Rgbkbd struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// NewRgbkbd connects to rgbkbd via D-Bus and returns a Rgbkbd object.
func NewRgbkbd(ctx context.Context) (*Rgbkbd, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Rgbkbd{conn, obj}, nil
}

// call is a thin wrapper over CallWithContext.
func (c *Rgbkbd) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// SetTestingMode allows the Rgbkbd daemon to write calls to a log file.
func (c *Rgbkbd) SetTestingMode(ctx context.Context, capability uint32) error {
	if err := c.call(ctx, "SetTestingMode", true, capability).Err; err != nil {
		return errors.Wrap(err, "failed to call method SetTestingMode")
	}
	return nil
}

// SetStaticBackgroundColor changes the keyboard backlight color.
func (c *Rgbkbd) SetStaticBackgroundColor(ctx context.Context, r, g, b uint8) error {
	if err := c.call(ctx, "SetStaticBackgroundColor", r, g, b).Err; err != nil {
		return errors.Wrap(err, "failed to call method SetStaticBackgroundColor")
	}
	return nil
}

// ResetRgbkbdState restores rgbkbd's initial state by resetting the keyboard
// to the default background color.
func ResetRgbkbdState(ctx context.Context, rgbkbd *Rgbkbd) error {
	// Reset to default background color. Overrides wallpaper extracted
	// color which varies depending on the device used.
	err := rgbkbd.SetStaticBackgroundColor(ctx, 255, 255, 210)
	if err != nil {
		return errors.Wrap(err, "Call to ResetRgbkbdState failed")
	}
	return nil
}

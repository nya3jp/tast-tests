// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// DeviceProperty is the type for device property names.
type DeviceProperty string

const (
	dbusDeviceInterface = "org.chromium.flimflam.Device"
)

// Device property names defined in dbus-constants.h .
const (
	// Device property names.
	DevicePropertyName DeviceProperty = "Name"
	DevicePropertyType DeviceProperty = "Type"
)

// Device wraps a Device D-Bus object in shill.
type Device struct {
	obj dbus.BusObject
}

// NewDevice connects to a device in Shill.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	s := &Device{obj: obj}
	return s, nil
}

// GetProperties returns a list of properties provided by the device.
func (d *Device) GetProperties(ctx context.Context) (map[DeviceProperty]interface{}, error) {
	props := make(map[DeviceProperty]interface{})
	if err := call(ctx, d.obj, dbusDeviceInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// SetProperty sets a string property to the given value
func (d *Device) SetProperty(ctx context.Context, property DeviceProperty, val interface{}) error {
	return call(ctx, d.obj, dbusDeviceInterface, "SetProperty", property, val).Err
}

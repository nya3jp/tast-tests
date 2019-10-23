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

const (
	dbusDeviceInterface = "org.chromium.flimflam.Device"
)

// DeviceProperty is the type for device property names.
type DeviceProperty string

// Device property names defined in dbus-constants.h .
const (
	// Device property names.
	DevicePropertyInterface DeviceProperty = "Interface"
	DevicePropertyType      DeviceProperty = "Type"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   DeviceProperty = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    DeviceProperty = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource DeviceProperty = "Ethernet.UsbEthernetMacAddressSource"
)

// Device wraps a Device D-Bus object in shill.
// It also caches device properties when GetProperties() is called.
type Device struct {
	obj   dbus.BusObject
	path  dbus.ObjectPath
	props map[DeviceProperty]interface{}
}

// NewDevice connects to shill's Device.
// It also obtains properties after device creation.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	m := &Device{obj: obj, path: path, props: make(map[DeviceProperty]interface{})}
	if _, err = m.GetProperties(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// String returns the path of the device.
// It is so named to conforms the Stringer interface.
func (d *Device) String() string {
	return string(d.path)
}

// GetProperties returns a list of properties provided by the device.
// Note that it is called in NewDevice(). Users can call again to refresh properties.
func (d *Device) GetProperties(ctx context.Context) (map[DeviceProperty]interface{}, error) {
	if err := call(ctx, d.obj, dbusDeviceInterface, "GetProperties").Store(&d.props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return d.props, nil
}

// GetStringProp returns the property value if the key exists and the value type is string.
func (d *Device) GetStringProp(key DeviceProperty) (string, error) {
	p, ok := d.props[key]
	if !ok {
		return "", errors.Errorf("shill device %s has no property %q", d.String(), key)
	}
	pv, ok := p.(string)
	if !ok {
		return "", errors.Errorf("Type of the property %q in the shill device %s is not string", key, d.String())
	}
	return pv, nil
}

// SetUsbEthernetMacAddressSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetUsbEthernetMacAddressSource(ctx context.Context, source string) error {
	if err := call(ctx, d.obj, dbusDeviceInterface, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}

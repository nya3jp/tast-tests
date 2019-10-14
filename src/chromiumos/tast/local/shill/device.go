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
	DevicePropertyAddress   DeviceProperty = "Address"
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
	conn  *dbus.Conn
	obj   dbus.BusObject
	path  dbus.ObjectPath
	props map[DeviceProperty]interface{}
}

// NewDevice connects to shill's Device.
// It also obtains properties after device creation.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	m := &Device{conn: conn, obj: obj, path: path, props: make(map[DeviceProperty]interface{})}
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

// DevicePropertyWatcher watches for device "PropertyChanged" signals.
type DevicePropertyWatcher struct {
	watcher *dbusutil.SignalWatcher
}

// WaitForExpectedChanges waits for expected propery value changes. Returns
// error if some property changed not as it was expected.
func (d *DevicePropertyWatcher) WaitForExpectedChanges(ctx context.Context, expectedChanges map[DeviceProperty]interface{}) error {
	for {
		select {
		case sig := <-d.watcher.Signals:
			if len(sig.Body) != 2 {
				return errors.Errorf("Signal body must contain 2 arguments: %v", sig.Body)
			}
			if prop, ok := sig.Body[0].(string); !ok {
				return errors.Errorf("Signal first argument must be a string: %v", sig.Body[0])
			} else if foundVal, ok := sig.Body[1].(dbus.Variant); !ok {
				return errors.Errorf("Signal second argument must be a variant: %v", sig.Body[1])
			} else if val, ok := expectedChanges[DeviceProperty(prop)]; !ok {
				continue
			} else if dbus.MakeVariant(val) != foundVal {
				return errors.Errorf("Property %s changed to %v, but %v was expected", prop, foundVal, val)
			} else {
				delete(expectedChanges, DeviceProperty(prop))
			}

			if len(expectedChanges) == 0 {
				return nil
			}

		case <-ctx.Done():
			return errors.Errorf("Didn't receive expected PropertyChanged signals %v due to %v", expectedChanges, ctx.Err())
		}
	}
}

// Close stops watching for signals.
func (d *DevicePropertyWatcher) Close(ctx context.Context) error {
	return d.watcher.Close(ctx)
}

// WatchPropertyChanged returns a SignalWatcher to observe the
// "PropertyChanged" signal.
func (d *Device) WatchPropertyChanged(ctx context.Context) (*DevicePropertyWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      d.path,
		Interface: dbusDeviceInterface,
		Member:    "PropertyChanged",
	}
	watcher, err := dbusutil.NewSignalWatcher(ctx, d.conn, spec)
	if err != nil {
		return nil, err
	}
	return &DevicePropertyWatcher{watcher: watcher}, err
}

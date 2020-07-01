// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
)

const (
	dbusDeviceInterface = "org.chromium.flimflam.Device"
)

// Device property names defined in dbus-constants.h .
const (
	// Device property names.
	DevicePropertyAddress         = "Address"
	DevicePropertyInterface       = "Interface"
	DevicePropertyType            = "Type"
	DevicePropertySelectedService = "SelectedService"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource = "Ethernet.UsbEthernetMacAddressSource"
	DevicePropertyEapDetected       = "EapAuthenticatorDetected"
	DevicePropertyEapCompleted      = "EapAuthenticationCompleted"

	// WiFi device property names.
	DevicePropertyWiFiBgscanMethod       = "BgscanMethod"
	DevicePropertyMACAddrRandomEnabled   = "MACAddressRandomizationEnabled"
	DevicePropertyMACAddrRandomSupported = "MACAddressRandomizationSupported"
	DevicePropertyScanning               = "Scanning" // Also for cellular.
)

// Device wraps a Device D-Bus object in shill.
type Device struct {
	PropertyHolder
}

// NewDevice connects to shill's Device.
// It also obtains properties after device creation.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	ph, err := NewPropertyHolder(ctx, dbusDeviceInterface, path)
	if err != nil {
		return nil, err
	}
	return &Device{PropertyHolder: ph}, nil
}

// SetUsbEthernetMacAddressSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetUsbEthernetMacAddressSource(ctx context.Context, source string) error {
	if err := d.dbusObject.Call(ctx, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}

// Enable enables the device.
func (d *Device) Enable(ctx context.Context) error {
	if err := d.dbusObject.Call(ctx, "Enable").Err; err != nil {
		return errors.Wrapf(err, "failed to enable device %s", d.String())
	}
	return nil
}

// Disable disables the device.
func (d *Device) Disable(ctx context.Context) error {
	if err := d.dbusObject.Call(ctx, "Disable").Err; err != nil {
		return errors.Wrapf(err, "failed to disable device %s", d.String())
	}
	return nil
}

// RequestRoam requests that we roam to the specified BSSID.
// Note: this operation assumes that:
// 1- We are connected to an SSID for wich |bssid| is a member.
// 2- There is a BSS with an appropriate ID in our scan results.
func (d *Device) RequestRoam(ctx context.Context, bssid string) error {
	if err := d.dbusObject.Call(ctx, "RequestRoam", bssid).Err; err != nil {
		return errors.Wrapf(err, "failed to roam %s", d.String())
	}
	return nil
}

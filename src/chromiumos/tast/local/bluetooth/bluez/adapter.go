// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluez contains helpers to interact with the system's bluetooth bluez
// adapters.
package bluez

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// Adapter contains helper functions for getting and setting bluetooth adapter
// state.
type Adapter struct {
	dbus *dbusutil.DBusObject
}

// NewAdapter creates a new bluetooth Adapter from the passed D-Bus object path.
func NewAdapter(ctx context.Context, path dbus.ObjectPath) (*Adapter, error) {
	obj, err := NewBluezDBusObject(ctx, bluezAdapterIface, path)
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		dbus: obj,
	}
	return a, nil
}

// Adapters creates an Adapter for all bluetooth adapters in the system.
func Adapters(ctx context.Context) ([]*Adapter, error) {
	paths, err := collectExistingBluezObjectPaths(ctx, bluezAdapterIface)
	if err != nil {
		return nil, err
	}
	adapters := make([]*Adapter, len(paths))
	for i, path := range paths {
		adapter, err := NewAdapter(ctx, path)
		if err != nil {
			return nil, err
		}
		adapters[i] = adapter
	}
	return adapters, nil
}

// DBusObject returns the D-Bus object wrapper for this object.
func (a *Adapter) DBusObject() *dbusutil.DBusObject {
	return a.dbus
}

// SetPowered turns a bluetooth adapter on or off
func (a *Adapter) SetPowered(ctx context.Context, powered bool) error {
	return a.dbus.SetProperty(ctx, "Powered", powered)
}

// Powered returns whether a bluetooth adapter is powered on.
func (a *Adapter) Powered(ctx context.Context) (bool, error) {
	return a.dbus.PropertyBool(ctx, "Powered")
}

// Address returns the MAC address of the adapter.
func (a *Adapter) Address(ctx context.Context) (string, error) {
	return a.dbus.PropertyString(ctx, "Address")
}

// Name returns the name of the adapter.
func (a *Adapter) Name(ctx context.Context) (string, error) {
	return a.dbus.PropertyString(ctx, "Name")
}

// Discoverable returns the discoverable property of the adapter.
func (a *Adapter) Discoverable(ctx context.Context) (bool, error) {
	return a.dbus.PropertyBool(ctx, "Discoverable")
}

// Discovering returns the discovering of the adapter.
func (a *Adapter) Discovering(ctx context.Context) (bool, error) {
	return a.dbus.PropertyBool(ctx, "Discovering")
}

// UUIDs returns the uuids of the adapter.
func (a *Adapter) UUIDs(ctx context.Context) ([]string, error) {
	return a.dbus.PropertyStrings(ctx, "UUIDs")
}

// Modalias returns the modalias of the adapter.
func (a *Adapter) Modalias(ctx context.Context) (string, error) {
	return a.dbus.PropertyString(ctx, "Modalias")
}

// StartDiscovery starts a discovery on the adapter.
func (a *Adapter) StartDiscovery(ctx context.Context) error {
	c := a.dbus.Call(ctx, "StartDiscovery")
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to start discovery")
	}
	return nil
}

// StopDiscovery stops the discovery on the adapter.
func (a *Adapter) StopDiscovery(ctx context.Context) error {
	c := a.dbus.Call(ctx, "StopDiscovery")
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to stop discovery")
	}
	return nil
}

// RemoveDevice removes the device with the specified path.
func (a *Adapter) RemoveDevice(ctx context.Context, devicePath dbus.ObjectPath) error {
	c := a.dbus.Call(ctx, "RemoveDevice", devicePath)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to remove device with path %q", devicePath)
	}
	return nil
}

// IsEnabled checks if bluetooth adapter present and powered on.
func IsEnabled(ctx context.Context) (bool, error) {
	adapters, err := Adapters(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return false, errors.Errorf("failed to verify the number of Bluetooth adapters got %d, expected 1 ", len(adapters))
	}
	return adapters[0].Powered(ctx)
}

// IsDisabled checks if bluetooth adapter present and powered off
func IsDisabled(ctx context.Context) (bool, error) {
	status, err := IsEnabled(ctx)
	return !status, err
}

// Enable powers on the bluetooth adapter
func Enable(ctx context.Context) error {
	adapters, err := Adapters(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return errors.Errorf("failed to verify the number of Bluetooth adapters got %d, expected 1 ", len(adapters))
	}
	return adapters[0].SetPowered(ctx, true)
}

// Disable powers off the bluetooth adapter.
func Disable(ctx context.Context) error {
	adapters, err := Adapters(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return errors.Errorf("failed to verify the number of Bluetooth adapters got %d, expected 1 ", len(adapters))
	}
	return adapters[0].SetPowered(ctx, false)
}

// PollForBTEnabled polls bluetooth adapter state till Adapter is powered on
func PollForBTEnabled(ctx context.Context) error {
	return PollForAdapterState(ctx, true)
}

// PollForBTDisabled polls bluetooth adapter state till Adapter is powered off
func PollForBTDisabled(ctx context.Context) error {
	return PollForAdapterState(ctx, false)
}

// PollForAdapterState polls bluetooth adapter state until expected state is received or timeout occurs.
func PollForAdapterState(ctx context.Context, exp bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		status, err := IsEnabled(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check Bluetooth status"))
		}
		if status != exp {
			return errors.Errorf("failed to verify Bluetooth status, got %t, want %t", status, exp)
		}

		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}

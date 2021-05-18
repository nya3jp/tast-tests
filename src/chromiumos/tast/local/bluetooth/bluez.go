// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluetooth contains helpers to interact with the system's bluetooth
// adapters.
package bluetooth

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// Adapter contains helper functions for getting and setting bluetooth adapter
// state.
type Adapter struct {
	obj  dbus.BusObject
	path dbus.ObjectPath
}

const service = "org.bluez"
const adapterIface = service + ".Adapter1"

// Adapters creates an Adapter for all bluetooth adapters in the system.
func Adapters(ctx context.Context) ([]*Adapter, error) {
	var adapters []*Adapter
	_, obj, err := dbusutil.Connect(ctx, service, "/")
	if err != nil {
		return nil, err
	}
	managed, err := dbusutil.ManagedObjects(ctx, obj)
	if err != nil {
		return nil, err
	}
	for _, path := range managed[adapterIface] {
		adapter, err := NewAdapter(ctx, path)
		if err != nil {
			return nil, err
		}
		adapters = append(adapters, adapter)
	}
	return adapters, nil
}

// NewAdapter creates a new bluetooth Adapter from the passed D-Bus object path.
func NewAdapter(ctx context.Context, path dbus.ObjectPath) (*Adapter, error) {
	_, obj, err := dbusutil.Connect(ctx, service, path)
	if err != nil {
		return nil, err
	}
	return &Adapter{obj, path}, nil
}

// Path gets the D-Bus path this adapter was created from.
func (a *Adapter) Path() dbus.ObjectPath {
	return a.path
}

const poweredProp = adapterIface + ".Powered"

// SetPowered turns a bluetooth adapter on or off
func (a *Adapter) SetPowered(ctx context.Context, powered bool) error {
	return dbusutil.SetProperty(ctx, a.obj, poweredProp, powered)
}

// Powered returns whether a bluetooth adapter is powered on.
func (a *Adapter) Powered(ctx context.Context) (bool, error) {
	value, err := dbusutil.Property(ctx, a.obj, poweredProp)
	if err != nil {
		return false, err
	}
	powered, ok := value.(bool)
	if !ok {
		return false, errors.New("powered property not a bool")
	}
	return powered, nil
}

const addressProp = adapterIface + ".Address"

// Address returns the MAC address of the adapter.
func (a *Adapter) Address(ctx context.Context) (string, error) {
	value, err := dbusutil.Property(ctx, a.obj, addressProp)
	if err != nil {
		return "", err
	}
	address, ok := value.(string)
	if !ok {
		return "", errors.New("address property not a string")
	}
	return address, nil
}

const nameProp = adapterIface + ".Name"

// Name returns the name of the adapter.
func (a *Adapter) Name(ctx context.Context) (string, error) {
	value, err := dbusutil.Property(ctx, a.obj, nameProp)
	if err != nil {
		return "", err
	}
	name, ok := value.(string)
	if !ok {
		return "", errors.New("name property not a string")
	}
	return name, nil
}

// StartDiscovery starts a discovery on the adapter.
func (a *Adapter) StartDiscovery(ctx context.Context) error {
	c := a.obj.CallWithContext(ctx, adapterIface+".StartDiscovery", 0)
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to start discovery")
	}
	return nil
}

// StopDiscovery stops the discovery on the adapter.
func (a *Adapter) StopDiscovery(ctx context.Context) error {
	c := a.obj.CallWithContext(ctx, adapterIface+".StopDiscovery", 0)
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to stop discovery")
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

// PollForBTEnabled polls bluetooth adapter state till Adapter is powered on
func PollForBTEnabled(ctx context.Context) error {
	return PollForAdapterState(ctx, true)
}

// PollForBTDisabled polls bluetooth adapter state till Adapter is powered off
func PollForBTDisabled(ctx context.Context) error {
	return PollForAdapterState(ctx, false)
}

//PollForAdapterState polls bluetooth adapter state until expected state is received or  timeout occurs.
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

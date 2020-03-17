// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluetooth contains helpers to interact with the system's bluetooth
// adapters.
package bluetooth

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Adapter contains helper functions for getting and setting bluetooth adapter
// state.
type Adapter struct {
	obj dbus.BusObject
}

const iface = "org.bluez.Adapter1"

// NewAdapter creates a new bluetooth Adapter from the passed D-Bus object path.
func NewAdapter(ctx context.Context, path string) (*Adapter, error) {
	const (
		name = "org.bluez"
	)
	_, obj, err := dbusutil.Connect(ctx, name, dbus.ObjectPath(path))
	if err != nil {
		return nil, err
	}
	return &Adapter{obj}, nil
}

const poweredProp = iface + ".Powered"

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

const addressProp = iface + ".Address"

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

const nameProp = iface + ".Name"

// Name returns the name of the adapter.
func (a *Adapter) Name(ctx context.Context) (string, error) {
	value, err := dbusutil.Property(ctx, a.obj, nameProp)
	if err != nil {
		return "", err
	}
	name, ok := value.(string)
	if !ok {
		return "", errors.New("powered property not a bool")
	}
	return name, nil
}

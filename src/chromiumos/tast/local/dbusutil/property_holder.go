// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"

	"github.com/godbus/dbus"
)

// PropertyHolder provides methods to access properties of a DBus object.
// The DBus object must provides GetProperties and SetProperty methods, and a PropertyChanged signal.
type PropertyHolder struct {
	*DBusObject
}

// NewPropertyHolder creates a DBus object with the given service, interface and path
// which can be used for accessing and setting properties.
func NewPropertyHolder(ctx context.Context, service, iface string, path dbus.ObjectPath) (*PropertyHolder, error) {
	dbusObject, err := NewDBusObject(ctx, service, iface, path)
	if err != nil {
		return nil, err
	}
	return &PropertyHolder{dbusObject}, nil
}

// CreateWatcher returns a PropertiesWatcher to observe the "PropertyChanged" signal.
func (h *PropertyHolder) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	watcher, err := h.DBusObject.CreateWatcher(ctx, "PropertyChanged")
	if err != nil {
		return nil, err
	}
	return &PropertiesWatcher{watcher: watcher}, nil
}

// GetProperties calls dbus GetProperties method on the interface and returns the result.
// The dbus call may fail with DBusErrorUnknownObject if the ObjectPath is not valid.
// Callers can check it with IsDBusError if it's expected.
func (h *PropertyHolder) GetProperties(ctx context.Context) (*Properties, error) {
	return NewProperties(ctx, h.DBusObject)
}

// GetDBusProperties calls the dbus GetAll method on the interface and returns the result.
func (h *PropertyHolder) GetDBusProperties(ctx context.Context) (*Properties, error) {
	return NewDBusProperties(ctx, h.DBusObject)
}

// SetProperty calls SetProperty on the interface to set property to the given value.
func (h *PropertyHolder) SetProperty(ctx context.Context, prop string, value interface{}) error {
	return h.Call(ctx, "SetProperty", prop, value).Err
}

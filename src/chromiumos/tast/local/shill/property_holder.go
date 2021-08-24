// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// PropertyHolder provides methods to access properties of shill via D-Bus.
// The interface for property get/set in shill is different from the general
// org.freedesktop.DBus.Properties, so we'll need to overwrite the
// {Get,Set}Properties, while the other utilities provided by dbusutil can
// still be reused.
type PropertyHolder struct {
	*dbusutil.PropertyHolder
}

// NewPropertyHolder creates a shill DBus object with the given service, interface and path
// which can be used for accessing and setting properties.
func NewPropertyHolder(ctx context.Context, service, iface string, path dbus.ObjectPath) (*PropertyHolder, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, service, iface, path)
	if err != nil {
		return nil, err
	}
	return &PropertyHolder{PropertyHolder: ph}, nil
}

// GetProperties calls GetProperties method of shill and return properties of the object.
func (h *PropertyHolder) GetProperties(ctx context.Context) (*dbusutil.Properties, error) {
	var props map[string]interface{}
	if err := h.Call(ctx, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrapf(err, "failed getting properties of %v", h)
	}
	return dbusutil.NewProperties(props), nil
}

// SetProperty calls SetProperties method of shill to set a property of the object.
func (h *PropertyHolder) SetProperty(ctx context.Context, prop string, value interface{}) error {
	return h.Call(ctx, "SetProperty", prop, value).Err
}

// GetAndSetProperty gets SetProperties method of shill to set a property of the object.
func (h *PropertyHolder) GetAndSetProperty(ctx context.Context, prop string, value interface{}) (interface{}, error) {
	properties, err := h.GetProperties(ctx); 
	if err!=nil {
		return nil, errors.Wrapf(err, "unable to get properties of %v", h);
	}
	curValue, err := properties.Get(prop)
	if err != nil {
		return curValue, errors.Wrapf(err, "unable to get %s", prop)
	}
	return curValue, h.SetProperty(ctx, prop, value)
}

// GetShillProperties calls GetProperties method of shill and return properties of the object.
// Deprecated: use GetProperties instead.
func (h *PropertyHolder) GetShillProperties(ctx context.Context) (*dbusutil.Properties, error) {
	return h.GetProperties(ctx)
}

// CreateWatcher returns a PropertiesWatcher to observe the object's "PropertyChanged" signal.
func (h *PropertyHolder) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	return NewPropertiesWatcher(ctx, h.DBusObject)
}

// WaitForProperty polls for the specified Shill property state to match |expected|.
func (h *PropertyHolder) WaitForProperty(ctx context.Context, property string, expected interface{}, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		props, err := h.GetProperties(ctx)
		if err != nil {
			return err
		}
		value, err := props.Get(property)
		if err != nil {
			return err
		}
		if value != expected {
			return errors.Errorf("unexpected property state for %q, got %v, expected %v", property, value, expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: 100 * time.Millisecond,
	})
}

// WaitForShillProperty polls for the specified Shill property state to match |expected|
// Deprecated: use WaitForProperty instead.
func (h *PropertyHolder) WaitForShillProperty(ctx context.Context, property string, expected interface{}, timeout time.Duration) error {
	return h.WaitForProperty(ctx, property, expected, timeout)
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"reflect"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// PropertiesWatcher watches for "PropertyChanged" signals.
type PropertiesWatcher struct {
	watcher *dbusutil.SignalWatcher
}

// Close stops watching for signals.
func (pw *PropertiesWatcher) Close(ctx context.Context) error {
	return pw.watcher.Close(ctx)
}

// Wait waits for "PropertyChanged" signal and updates corresponding property value.
func (pw *PropertiesWatcher) Wait(ctx context.Context) (string, interface{}, dbus.Sequence, error) {
	select {
	case sig := <-pw.watcher.Signals:
		if len(sig.Body) != 2 {
			return "", nil, 0, errors.Errorf("signal body must contain 2 arguments: %v", sig.Body)
		}
		if prop, ok := sig.Body[0].(string); !ok {
			return "", nil, 0, errors.Errorf("signal first argument must be a string: %v", sig.Body[0])
		} else if variant, ok := sig.Body[1].(dbus.Variant); !ok {
			return "", nil, 0, errors.Errorf("signal second argument must be a variant: %v", sig.Body[1])
		} else {
			return prop, variant.Value(), sig.Sequence, nil
		}
	case <-ctx.Done():
		return "", nil, 0, errors.Errorf("didn't receive PropertyChanged signal: %v", ctx.Err())
	}
}

// WaitAll waits for all expected properties were shown on at least one "PropertyChanged" signal and returns the last updated
// value of each property.
func (pw *PropertiesWatcher) WaitAll(ctx context.Context, props ...string) ([]interface{}, error) {
	values := make([]interface{}, len(props))
	seen := make([]bool, len(props))
	unseen := len(props)

	for unseen > 0 {
		prop, val, _, err := pw.Wait(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to wait for any property: %q", props)
		}
		for i, p := range props {
			if p != prop {
				continue
			}
			if !seen[i] {
				seen[i] = true
				unseen--
			}
			values[i] = val
		}
	}
	return values, nil
}

// Expect waits for the expected value of a given property.
func (pw *PropertiesWatcher) Expect(ctx context.Context, prop string, expected interface{}) error {
	_, err := pw.ExpectIn(ctx, prop, []interface{}{expected})
	return err
}

// ExpectIn expects the prop's value to become one of the expected values and returns the first matched one.
func (pw *PropertiesWatcher) ExpectIn(ctx context.Context, prop string, expected []interface{}) (interface{}, error) {
	for {
		vals, err := pw.WaitAll(ctx, prop)
		if err != nil {
			return nil, err
		}
		for _, e := range expected {
			if reflect.DeepEqual(e, vals[0]) {
				return vals[0], nil
			}
		}
	}
}

// PropertyHolder provides methods to access properties of a DBus object.
// The DBus object must provides GetProperties and SetProperty methods, and a PropertyChanged signal.
type PropertyHolder struct {
	dbusObject *dbusutil.DBusObject
}

// NewPropertyHolder creates a DBus object with the given path and interface used for accessing properties.
func NewPropertyHolder(ctx context.Context, iface string, path dbus.ObjectPath) (PropertyHolder, error) {
	dbusObject, err := dbusutil.NewDBusObject(ctx, dbusService, iface, path)
	if err != nil {
		return PropertyHolder{}, err
	}
	return PropertyHolder{dbusObject: dbusObject}, nil
}

// CreateWatcher returns a PropertiesWatcher to observe the "PropertyChanged" signal.
func (h *PropertyHolder) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	watcher, err := h.dbusObject.CreateWatcher(ctx, "PropertyChanged")
	if err != nil {
		return nil, err
	}
	return &PropertiesWatcher{watcher: watcher}, nil
}

// GetProperties calls dbus GetProperties method on the interface and returns the result.
// The dbus call may fail with dbusutil.DBusErrorUnknownObject if the ObjectPath is not valid.
// Callers can check it with dbusutil.IsDBusError if it's expected.
func (h *PropertyHolder) GetProperties(ctx context.Context) (*dbusutil.Properties, error) {
	return dbusutil.NewProperties(ctx, h.dbusObject)
}

// ObjectPath returns the underlying object's D-Bus path.
func (h *PropertyHolder) ObjectPath() dbus.ObjectPath {
	return h.dbusObject.ObjectPath()
}

// String return the string of underlying dbusObject.
func (h *PropertyHolder) String() string {
	return h.dbusObject.String()
}

// SetProperty calls SetProperty on the interface to set property to the given value.
func (h *PropertyHolder) SetProperty(ctx context.Context, prop string, value interface{}) error {
	return h.dbusObject.Call(ctx, "SetProperty", prop, value).Err
}

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

// Properties wraps shill D-Bus object properties.
type Properties struct {
	props map[string]interface{}
}

// NewProperties creates a new Properties object and populates it with shill's object properties.
func NewProperties(ctx context.Context, d *DBusObject) (*Properties, error) {
	var props map[string]interface{}
	if err := d.Call(ctx, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrapf(err, "failed getting properties of %v", d)
	}
	return &Properties{props: props}, nil
}

// Has returns whether property exist.
func (p *Properties) Has(prop string) bool {
	_, ok := p.props[prop]
	return ok
}

// Get returns property value.
func (p *Properties) Get(prop string) (interface{}, error) {
	value, ok := p.props[prop]
	if !ok {
		return nil, errors.Errorf("property %s does not exist", prop)
	}
	return value, nil
}

// GetString returns string property value.
func (p *Properties) GetString(prop string) (string, error) {
	value, err := p.Get(prop)
	if err != nil {
		return "", err
	}
	str, ok := value.(string)
	if !ok {
		return "", errors.Errorf("property %s is not a string: %q", prop, value)
	}
	return str, nil

}

// GetStrings returns property value as string array.
func (p *Properties) GetStrings(prop string) ([]string, error) {
	value, err := p.Get(prop)
	if err != nil {
		return nil, err
	}
	str, ok := value.([]string)
	if !ok {
		return nil, errors.Errorf("property %s is not a string array: %q", prop, value)
	}
	return str, nil

}

// GetBool returns the property value as a boolean.
func (p *Properties) GetBool(prop string) (bool, error) {
	value, err := p.Get(prop)
	if err != nil {
		return false, err
	}
	b, ok := value.(bool)
	if !ok {
		return false, errors.Errorf("property %s is not a bool: %q", prop, value)
	}
	return b, nil

}

// GetObjectPath returns the DBus ObjectPath of the given property name.
func (p *Properties) GetObjectPath(prop string) (dbus.ObjectPath, error) {
	value, err := p.Get(prop)
	if err != nil {
		return dbus.ObjectPath(""), err
	}
	objPath, ok := value.(dbus.ObjectPath)
	if !ok {
		return dbus.ObjectPath(""), errors.Errorf("property %s is not a dbus.ObjectPath: %q", prop, value)
	}
	return objPath, nil
}

// GetObjectPaths returns the list of DBus ObjectPaths of the given property name.
func (p *Properties) GetObjectPaths(prop string) ([]dbus.ObjectPath, error) {
	value, err := p.Get(prop)
	if err != nil {
		return nil, err
	}
	objPaths, ok := value.([]dbus.ObjectPath)
	if !ok {
		return nil, errors.Errorf("property %s is not a list of dbus.ObjectPath: %q", prop, value)
	}
	return objPaths, nil
}

// PropertiesWatcher watches for "PropertyChanged" signals.
type PropertiesWatcher struct {
	watcher *dbusutil.SignalWatcher
}

// Close stops watching for signals.
func (pw *PropertiesWatcher) Close(ctx context.Context) error {
	return pw.watcher.Close(ctx)
}

// Wait waits for "PropertyChanged" signal and updates corresponding property value.
func (pw *PropertiesWatcher) Wait(ctx context.Context) (string, interface{}, error) {
	select {
	case sig := <-pw.watcher.Signals:
		if len(sig.Body) != 2 {
			return "", nil, errors.Errorf("signal body must contain 2 arguments: %v", sig.Body)
		}
		if prop, ok := sig.Body[0].(string); !ok {
			return "", nil, errors.Errorf("signal first argument must be a string: %v", sig.Body[0])
		} else if variant, ok := sig.Body[1].(dbus.Variant); !ok {
			return "", nil, errors.Errorf("signal second argument must be a variant: %v", sig.Body[1])
		} else {
			return prop, variant.Value(), nil
		}
	case <-ctx.Done():
		return "", nil, errors.Errorf("didn't receive PropertyChanged signal: %v", ctx.Err())
	}
}

// WaitAll waits for all expected properties were shown on at least one "PropertyChanged" signal and returns the last updated
// value of each property.
func (pw *PropertiesWatcher) WaitAll(ctx context.Context, props ...string) ([]interface{}, error) {
	values := make([]interface{}, len(props))
	seen := make([]bool, len(props))
	unseen := len(props)

	for unseen > 0 {
		prop, val, err := pw.Wait(ctx)
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

// PropertyHolder provides methods to access properties of a DBus object.
// The DBus object must provides GetProperties and SetProperty methods, and a PropertyChanged signal.
type PropertyHolder struct {
	dbusObject *DBusObject
}

// NewPropertyHolder creates a DBus object with the given path and interface used for accessing properties.
func NewPropertyHolder(ctx context.Context, iface string, path dbus.ObjectPath) (PropertyHolder, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return PropertyHolder{}, err
	}
	return PropertyHolder{
		dbusObject: &DBusObject{iface: iface, obj: obj, conn: conn},
	}, nil
}

// CreateWatcher returns a PropertiesWatcher to observe the "PropertyChanged" signal.
func (h *PropertyHolder) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      h.dbusObject.obj.Path(),
		Interface: h.dbusObject.iface,
		Member:    "PropertyChanged",
	}
	watcher, err := dbusutil.NewSignalWatcher(ctx, h.dbusObject.conn, spec)
	if err != nil {
		return nil, err
	}
	return &PropertiesWatcher{watcher: watcher}, nil
}

// GetProperties calls GetProperties on the interface and returns the result.
func (h *PropertyHolder) GetProperties(ctx context.Context) (*Properties, error) {
	return NewProperties(ctx, h.dbusObject)
}

// String return the string of underlying dbusObject.
func (h *PropertyHolder) String() string {
	return h.dbusObject.String()
}

// SetProperty calls SetProperty on the interface to set property to the given value.
func (h *PropertyHolder) SetProperty(ctx context.Context, prop string, value interface{}) error {
	return h.dbusObject.Call(ctx, "SetProperty", prop, value).Err
}

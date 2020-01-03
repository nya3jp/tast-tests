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
	dbusObject *DBusObject
	props      map[string]interface{}
}

// NewProperties fetches shill's object properties.
func NewProperties(ctx context.Context, d *DBusObject) (*Properties, error) {
	var props map[string]interface{}
	if err := d.Call(ctx, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrapf(err, "failed getting properties of %v", d)
	}
	return &Properties{dbusObject: d, props: props}, nil
}

func (p *Properties) set(prop string, value interface{}) {
	p.props[prop] = value
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

// SetProperty sets a property to the given value.
// It also writes the property to Properties' associates D-Bus object.
func (p *Properties) SetProperty(ctx context.Context, prop string, value interface{}) error {
	err := p.dbusObject.Call(ctx, "SetProperty", prop, value).Err
	if err == nil {
		p.set(prop, value)
	}
	return err
}

// GetBool returns bool property value.
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
	props   *Properties
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
			pw.props.set(prop, variant.Value())
			return prop, variant.Value(), nil
		}
	case <-ctx.Done():
		return "", nil, errors.Errorf("didn't receive PropertyChanged signal: %v", ctx.Err())
	}
}

// WaitAll waits for all expected properties were shown on at least one "PropertyChanged" signal and updates corresponding properties.
func (pw *PropertiesWatcher) WaitAll(ctx context.Context, props ...string) error {
	for {
		if len(props) == 0 {
			return nil
		}

		prop, _, err := pw.Wait(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to wait for any property: %q", props)
		}
		for i, p := range props {
			if p == prop {
				props = append(props[:i], props[i+1:]...)
				break
			}
		}
	}
}

// CreateWatcher returns a SignalWatcher to observe the "PropertyChanged" signal.
func (p *Properties) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      p.dbusObject.obj.Path(),
		Interface: p.dbusObject.iface,
		Member:    "PropertyChanged",
	}
	watcher, err := dbusutil.NewSignalWatcher(ctx, p.dbusObject.conn, spec)
	if err != nil {
		return nil, err
	}
	return &PropertiesWatcher{props: p, watcher: watcher}, err
}

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
	dbus *DBus
	m    map[string]interface{}
}

// NewProperties fetches shill's object properties.
func NewProperties(ctx context.Context, d *DBus) (*Properties, error) {
	var variantM map[string]dbus.Variant
	if err := call(ctx, d.Object, d.Interface, "GetProperties").Store(&variantM); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}

	m := make(map[string]interface{}, 0)
	for prop, variant := range variantM {
		m[prop] = variant.Value()
	}
	return &Properties{dbus: d, m: m}, nil
}

func (p *Properties) set(prop string, value interface{}) {
	p.m[prop] = value
}

// Has returns whether property exist.
func (p *Properties) Has(prop string) bool {
	_, ok := p.m[prop]
	return ok
}

// Get returns property value.
func (p *Properties) Get(prop string) interface{} {
	return p.m[prop]
}

// GetString returns string property value.
func (p *Properties) GetString(prop string) (string, error) {
	if value, ok := p.m[prop]; !ok {
		return "", errors.Errorf("property %s does not exist", prop)
	} else if str, ok := value.(string); !ok {
		return "", errors.Errorf("property %s is not a string: %q", prop, value)
	} else {
		return str, nil
	}
}

// GetObjectPaths returns a list of DBus ObjectPaths which is associated with the given property name.
func (p *Properties) GetObjectPaths(prop string) ([]dbus.ObjectPath, error) {
	if value, ok := p.m[prop]; !ok {
		return nil, errors.Errorf("property %s does not exist", prop)
	} else if result, ok := value.([]dbus.ObjectPath); !ok {
		return nil, errors.Errorf("property %s is not a list of dbus.ObjectPath: %q", prop, value)
	} else {
		return result, nil
	}
}

// PropertiesWatcher watches for "PropertyChanged" signals.
type PropertiesWatcher struct {
	props   *Properties
	watcher *dbusutil.SignalWatcher
}

// Close stops watching for signals.
func (p *PropertiesWatcher) Close(ctx context.Context) error {
	return p.watcher.Close(ctx)
}

// Wait waits for "PropertyChanged" signal and updates appropriate property value.
func (p *PropertiesWatcher) Wait(ctx context.Context) (string, interface{}, error) {
	select {
	case sig := <-p.watcher.Signals:
		if len(sig.Body) != 2 {
			return "", nil, errors.Errorf("signal body must contain 2 arguments: %v", sig.Body)
		}
		if prop, ok := sig.Body[0].(string); !ok {
			return "", nil, errors.Errorf("signal first argument must be a string: %v", sig.Body[0])
		} else if variant, ok := sig.Body[1].(dbus.Variant); !ok {
			return "", nil, errors.Errorf("signal second argument must be a variant: %v", sig.Body[1])
		} else {
			p.props.set(prop, variant.Value())
			return prop, variant.Value(), nil
		}
	case <-ctx.Done():
		return "", nil, errors.Errorf("didn't receive PropertyChanged signal: %v", ctx.Err())
	}
}

// WaitAll waits for at least one "PropertyChanged" signal for each passed property and updates appropriate properties.
func (p *PropertiesWatcher) WaitAll(ctx context.Context, props ...string) error {
	for {
		if len(props) == 0 {
			return nil
		}

		prop, _, err := p.Wait(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to wait for any property: %q", props)
		}
		for i, p := range props {
			if p == prop {
				props = append(props[:i], props[i+1:]...)
			}
		}
	}
}

// CreateWatcher returns a SignalWatcher to observe the "PropertyChanged" signal.
func (p *Properties) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      p.dbus.Object.Path(),
		Interface: p.dbus.Interface,
		Member:    "PropertyChanged",
	}
	watcher, err := dbusutil.NewSignalWatcher(ctx, p.dbus.Conn, spec)
	if err != nil {
		return nil, err
	}
	return &PropertiesWatcher{props: p, watcher: watcher}, err
}

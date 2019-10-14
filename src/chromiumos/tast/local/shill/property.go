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
	if err := call(ctx, d.Object, d.Interface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
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

// GetString returns string property value.
func (p *Properties) GetString(prop string) (string, error) {
	if value, err := p.Get(prop); err != nil {
		return "", err
	} else if str, ok := value.(string); !ok {
		return "", errors.Errorf("property %s is not a string: %q", prop, value)
	} else {
		return str, nil
	}
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
		Path:      p.dbusObject.Object.Path(),
		Interface: p.dbusObject.Interface,
		Member:    "PropertyChanged",
	}
	watcher, err := dbusutil.NewSignalWatcher(ctx, p.dbusObject.Conn, spec)
	if err != nil {
		return nil, err
	}
	return &PropertiesWatcher{props: p, watcher: watcher}, err
}

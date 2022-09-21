// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
)

// Properties wraps D-Bus object properties.
type Properties struct {
	props map[string]interface{}
}

// NewProperties creates a new Properties object with raw data.
func NewProperties(p map[string]interface{}) *Properties {
	return &Properties{props: p}
}

// NewDBusProperties creates a new Properties object and populates it with the
// object's properties using org.freedesktop.DBus.Properties.GetAll.
// The dbus call may fail with DBusErrorUnknownObject if the DBusObject is not valid.
// Callers can us IsDBusError to test for that case.
func NewDBusProperties(ctx context.Context, d *DBusObject) (*Properties, error) {
	props, err := d.AllProperties(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting all properties of %v", d)
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

// GetMap returns the property value as nested set of properties.
func (p *Properties) GetMap(prop string) (*Properties, error) {
	value, err := p.Get(prop)
	if err != nil {
		return nil, err
	}
	ret, ok := value.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("property %s is not a map: %q", prop, value)
	}
	return NewProperties(ret), nil
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

// GetUint8 returns the property value as uint8.
func (p *Properties) GetUint8(prop string) (uint8, error) {
	value, err := p.Get(prop)
	if err != nil {
		return 0, err
	}
	ret, ok := value.(uint8)
	if !ok {
		return 0, errors.Errorf("property %s is not an uint8: %q", prop, value)
	}
	return ret, nil
}

// GetUint8s returns the property value as uint8 array.
func (p *Properties) GetUint8s(prop string) ([]uint8, error) {
	value, err := p.Get(prop)
	if err != nil {
		return nil, err
	}
	ret, ok := value.([]uint8)
	if !ok {
		return nil, errors.Errorf("property %s is not a uint8 array: %q", prop, value)
	}
	return ret, nil
}

// GetUint16 returns the property value as uint16.
func (p *Properties) GetUint16(prop string) (uint16, error) {
	value, err := p.Get(prop)
	if err != nil {
		return 0, err
	}
	ret, ok := value.(uint16)
	if !ok {
		return 0, errors.Errorf("property %s is not an uint16: %q", prop, value)
	}
	return ret, nil
}

// GetUint16s returns the property value as uint16 array.
func (p *Properties) GetUint16s(prop string) ([]uint16, error) {
	value, err := p.Get(prop)
	if err != nil {
		return nil, err
	}
	ret, ok := value.([]uint16)
	if !ok {
		return nil, errors.Errorf("property %s is not an uint16 array: %q", prop, value)
	}
	return ret, nil
}

// GetInt32 returns the property value as int32.
func (p *Properties) GetInt32(prop string) (int32, error) {
	value, err := p.Get(prop)
	if err != nil {
		return 0, err
	}
	ret, ok := value.(int32)
	if !ok {
		return 0, errors.Errorf("property %s is not an int32: %q", prop, value)
	}
	return ret, nil
}

// GetUint32 returns the property value as a uint32.
func (p *Properties) GetUint32(prop string) (uint32, error) {
	value, err := p.Get(prop)
	if err != nil {
		return 0, err
	}
	ret, ok := value.(uint32)
	if !ok {
		return 0, errors.Errorf("property %s is not a uint32: %q", prop, value)
	}
	return ret, nil
}

// GetFloat64 returns the property value as float64.
func (p *Properties) GetFloat64(prop string) (float64, error) {
	value, err := p.Get(prop)
	if err != nil {
		return 0, err
	}
	ret, ok := value.(float64)
	if !ok {
		return 0, errors.Errorf("property %s is not a float64: %q", prop, value)
	}
	return ret, nil
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

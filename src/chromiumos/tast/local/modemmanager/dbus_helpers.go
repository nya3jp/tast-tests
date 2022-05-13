// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/local/dbusutil"
)

// Dbus Helper types and functions to simplify multi step DBus calls.

// Properties wraps a dbusutil.Properties object and the error result when creating the object .
type Properties struct {
	properties *dbusutil.Properties
	err        error
}

// Interface wraps an interface object and the error result when creating the object .
type Interface struct {
	iface interface{}
	err   error
}

// ObjectPath wraps a dbus.ObjectPath object and the error result when creating the object .
type ObjectPath struct {
	objectPath dbus.ObjectPath
	err        error
}

// ObjectPaths wraps a []dbus.ObjectPath object and the error result when creating the object .
type ObjectPaths struct {
	objectPaths []dbus.ObjectPath
	err         error
}

// PropertyHolder wraps a dbusutil.PropertyHolder object and the error result when creating the object .
type PropertyHolder struct {
	propertyHolder *dbusutil.PropertyHolder
	err            error
}

// Get returns property value.
func (i Properties) Get(prop string) Interface {
	if i.err != nil {
		return Interface{nil, i.err}
	}
	val, err := i.properties.Get(prop)
	return Interface{val, err}
}

// GetObjectProperties creates a new PropertyHolder and returns its properties.
// If the ObjectPath already contains an error, returns early.
func (i ObjectPath) GetObjectProperties(ctx context.Context, service, iface string) Properties {
	if i.err != nil {
		return Properties{nil, i.err}
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, service, iface, i.objectPath)
	if err != nil {
		return Properties{nil, err}
	}
	val, err := ph.GetProperties(ctx)
	return Properties{val, err}
}

// GetObjectPath returns the DBus ObjectPath of the given property name.
// If the Properties already contains an error, returns early.
func (i Properties) GetObjectPath(prop string) ObjectPath {
	if i.err != nil {
		return ObjectPath{"", i.err}
	}
	val, err := i.properties.GetObjectPath(prop)
	return ObjectPath{val, err}
}

// GetObjectPaths returns the list of DBus ObjectPaths of the given property name.
// If the Properties already contains an error, returns early.
func (i Properties) GetObjectPaths(prop string) ObjectPaths {
	if i.err != nil {
		return ObjectPaths{nil, i.err}
	}
	val, err := i.properties.GetObjectPaths(prop)
	return ObjectPaths{val, err}
}

// GetProperties calls dbus.GetProperties with the PropertyHolder object and returns the result.
// If the PropertyHolder already contains an error, returns early.
func (i PropertyHolder) GetProperties(ctx context.Context) Properties {
	if i.err != nil {
		return Properties{nil, i.err}
	}
	val, err := i.propertyHolder.GetProperties(ctx)
	return Properties{val, err}
}

// GetPropertyHolder creates a new PropertyHolder and returns the result.
// If the ObjectPath already contains an error, returns early.
func (i ObjectPath) GetPropertyHolder(ctx context.Context, service, iface string) PropertyHolder {
	if i.err != nil {
		return PropertyHolder{nil, i.err}
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, service, iface, i.objectPath)
	if err != nil {
		return PropertyHolder{nil, err}
	}
	return PropertyHolder{ph, nil}
}

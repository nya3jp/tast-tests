// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

// Dbus Helper types and functions to simplify multi step DBus calls.
type eProperties struct {
	properties *dbusutil.Properties
	err        error
}
type eInterface struct {
	iface interface{}
	err   error
}
type eObjectPath struct {
	objectPath dbus.ObjectPath
	err        error
}
type eObjectPaths struct {
	objectPaths []dbus.ObjectPath
	err         error
}
type ePropertyHolder struct {
	propertyHolder *dbusutil.PropertyHolder
	err            error
}

func (i eProperties) gGet(prop string) eInterface {
	if i.err != nil {
		return eInterface{nil, i.err}
	}
	val, err := i.properties.Get(prop)
	return eInterface{val, err}
}

func (i eObjectPath) gGetObjectProperties(ctx context.Context, service, iface string) eProperties {
	if i.err != nil {
		return eProperties{nil, i.err}
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, service, iface, i.objectPath)
	if err != nil {
		return eProperties{nil, err}
	}
	val, err := ph.GetProperties(ctx)
	return eProperties{val, err}
}

func (i eProperties) gGetObjectPath(prop string) eObjectPath {
	if i.err != nil {
		return eObjectPath{"", i.err}
	}
	val, err := i.properties.GetObjectPath(prop)
	return eObjectPath{val, err}
}

func (i eProperties) gGetObjectPaths(prop string) eObjectPaths {
	if i.err != nil {
		return eObjectPaths{nil, i.err}
	}
	val, err := i.properties.GetObjectPaths(prop)
	return eObjectPaths{val, err}
}

func (i ePropertyHolder) gGetProperties(ctx context.Context) eProperties {
	if i.err != nil {
		return eProperties{nil, i.err}
	}
	val, err := i.propertyHolder.GetProperties(ctx)
	return eProperties{val, err}
}

func (i eObjectPath) gGetPropertyHolder(ctx context.Context, service, iface string) ePropertyHolder {
	if i.err != nil {
		return ePropertyHolder{nil, i.err}
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, service, iface, i.objectPath)
	if err != nil {
		return ePropertyHolder{nil, err}
	}
	return ePropertyHolder{ph, nil}
}

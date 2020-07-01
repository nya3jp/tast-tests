// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	dbusInterfaceInterface = "fi.w1.wpa_supplicant1.Interface"
	dbusInterfacePropBSSs  = "BSSs"
	// DBusInterfaceSignalBSSAdded Interface became awaere of a new BSS.
	DBusInterfaceSignalBSSAdded = "BSSAdded"
	// BSSAddedSignalPropSSID  BSS SSID property name.
	BSSAddedSignalPropSSID = "SSID"
	// BSSAddedSignalPropBSSID  BSS BSSID property name.
	BSSAddedSignalPropBSSID = "BSSID"
)

// Interface is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1.Interface interface.
type Interface struct {
	dbus *DBusObject
}

// NewInterface creates an Interface object.
func NewInterface(ctx context.Context, path dbus.ObjectPath) (*Interface, error) {
	d, err := NewDBusObject(ctx, path, dbusInterfaceInterface)
	if err != nil {
		return nil, err
	}
	return &Interface{dbus: d}, nil
}

// BSSs returns the BSSs property of the interface.
func (iface *Interface) BSSs(ctx context.Context) ([]*BSS, error) {
	var bssPaths []dbus.ObjectPath
	if err := iface.dbus.Get(ctx, dbusInterfacePropBSSs, &bssPaths); err != nil {
		return nil, err
	}
	var ret []*BSS
	for _, path := range bssPaths {
		bss, err := NewBSS(ctx, path)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to NewBSS(%s): %v", path, err)
		} else {
			ret = append(ret, bss)
		}
	}
	return ret, nil
}

// DBusObject returns the D-Bus object of the interface.
func (iface *Interface) DBusObject() *DBusObject {
	return iface.dbus
}

// BSSAddedSignal wraps D-Bus BSSAdded signal arguments.
type BSSAddedSignal struct {
	BSS   dbus.ObjectPath
	Props map[string]interface{}
}

// ParseBSSAddedSignal returns the BSSAdded signal arguments. It only add the specified properties to the BSSAddedSignal struct.
func (iface *Interface) ParseBSSAddedSignal(ctx context.Context, sig *dbus.Signal, props []string) (*BSSAddedSignal, error) {
	if len(sig.Body) != 2 {
		return nil, errors.Errorf("got body length=%d, want 2", len(sig.Body))
	}

	path, ok := sig.Body[0].(dbus.ObjectPath)
	if !ok {
		return nil, errors.Errorf("got body %v, want dbus.ObjectPath", sig.Body[0])
	}

	bssProps, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, errors.Errorf("got body %v, want map[string]dbus.Variant", sig.Body[1])
	}

	values := make(map[string]interface{})
	for _, p := range props {
		val, ok := bssProps[p]
		if !ok {
			return nil, errors.Errorf("failed to find the value of the property %s", p)
		}
		values[p] = val.Value()
	}

	return &BSSAddedSignal{
		BSS:   path,
		Props: values,
	}, nil
}

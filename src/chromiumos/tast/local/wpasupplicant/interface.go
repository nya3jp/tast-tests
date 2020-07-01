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
	SSID  []byte
	BSSID []byte
}

// ParseBSSAddedSignal parses the D-Bus signal to a BSSAddedSignal.
func (iface *Interface) ParseBSSAddedSignal(ctx context.Context, sig *dbus.Signal) (*BSSAddedSignal, error) {
	if len(sig.Body) != 2 {
		return nil, errors.Errorf("len(sig.Body)=%d, want 2", len(sig.Body))
	}

	path, ok := sig.Body[0].(dbus.ObjectPath)
	if !ok {
		return nil, errors.Errorf("got sig.Body[0]: %v, want dbus.ObjectPath type", sig.Body[0])
	}

	bssProps, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, errors.Errorf("got sig.Body[1]: %v, want map[string]dbus.Variant type", sig.Body[1])
	}

	ssid, ok := bssProps["SSID"]
	if !ok {
		return nil, errors.New("failed to find the value of the property SSID")
	}

	bssid, ok := bssProps["BSSID"]
	if !ok {
		return nil, errors.New("failed to find the value of the property BSSID")
	}

	bSSID, ok := ssid.Value().([]byte)
	if !ok {
		return nil, errors.Errorf("got SSID %v, want []byte", ssid.Value())
	}

	bBSSID, ok := bssid.Value().([]byte)
	if !ok {
		return nil, errors.Errorf("got BSSID %v, want []byte", bssid.Value())
	}

	return &BSSAddedSignal{
		BSS:   path,
		SSID:  bSSID,
		BSSID: bBSSID,
	}, nil
}

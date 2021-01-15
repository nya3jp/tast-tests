// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpasupplicant

import (
	"context"
	"strings"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	dbusInterfaceInterface      = "fi.w1.wpa_supplicant1.Interface"
	dbusInterfaceMethodFlushBSS = "FlushBSS"
	dbusInterfaceMethodReattach = "Reattach"
	dbusInterfacePropBSSs       = "BSSs"
	// DBusInterfacePropDisconnectReason the most recent IEEE802.11 reason code for disconnect. Negative value indicates locally generated disconnect.
	DBusInterfacePropDisconnectReason = "DisconnectReason"
	// DBusInterfaceSignalBSSAdded Interface became awaere of a new BSS.
	DBusInterfaceSignalBSSAdded = "BSSAdded"
	// DBusInterfaceSignalPropertiesChanged indicates that some properties have changed. Possible properties are: "ApScan", "Scanning", "State", "CurrentBSS", "CurrentNetwork".
	DBusInterfaceSignalPropertiesChanged = "PropertiesChanged"
	// DBusInterfaceSignalScanDone indicates that the scanning is finished.
	DBusInterfaceSignalScanDone = "ScanDone"
	// DBusInterfaceSignalEAP indicates the status of the EAP peer.
	DBusInterfaceSignalEAP = "EAP"

	// DBusInterfaceStateAssociated is the value of the State property when the interface is associated.
	DBusInterfaceStateAssociated = "associated"
	// DBusInterfaceStateCompleted is the value of the State property when all authentication is completed.
	DBusInterfaceStateCompleted = "completed"
)

// Interface is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1.Interface interface.
type Interface struct {
	dbus *dbusutil.DBusObject
}

// NewInterface creates an Interface object.
func NewInterface(ctx context.Context, path dbus.ObjectPath) (*Interface, error) {
	d, err := dbusutil.NewDBusObject(ctx, dbusBaseInterface, dbusInterfaceInterface, path)
	if err != nil {
		return nil, err
	}
	return &Interface{dbus: d}, nil
}

// BSSs returns the BSSs property of the interface.
func (iface *Interface) BSSs(ctx context.Context) ([]*BSS, error) {
	ctx, st := timing.Start(ctx, "interface.BSSs")
	defer st.End()

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
func (iface *Interface) DBusObject() *dbusutil.DBusObject {
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

// ParseScanDoneSignal parses the ScanDone D-Bus signal and returns if it is a
// successful ScanDone.
func (iface *Interface) ParseScanDoneSignal(ctx context.Context, sig *dbus.Signal) (bool, error) {
	// Checks if it's a successful ScanDone.
	if len(sig.Body) != 1 {
		return false, errors.Errorf("got body length=%d, want 1", len(sig.Body))
	}
	b, ok := sig.Body[0].(bool)
	if !ok {
		return false, errors.Errorf("got body %v, want boolean", sig.Body[0])
	}
	return b, nil
}

// Reattach calls the Reattach method of the interface.
func (iface *Interface) Reattach(ctx context.Context) error {
	return iface.dbus.Call(ctx, dbusInterfaceMethodReattach).Err
}

// SignalName returns the name of the dbus.Signal, which may be one of DBusInterfaceSignal*.
func SignalName(s *dbus.Signal) string {
	parts := strings.Split(s.Name, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// FlushBSS calls the FlushBSS method of the interface to flush BSS entries from the cache.
func (iface *Interface) FlushBSS(ctx context.Context, age uint32) error {
	return iface.dbus.Call(ctx, dbusInterfaceMethodFlushBSS, age).Err
}

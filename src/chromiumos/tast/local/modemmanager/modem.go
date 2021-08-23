// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// Modem wraps a Modemmanager.Modem D-Bus object.
type Modem struct {
	*dbusutil.PropertyHolder
}

// NewModem creates a new PropertyHolder instance for the Modem object.
func NewModem(ctx context.Context) (*Modem, error) {
	_, obj, err := dbusutil.ConnectNoTiming(ctx, DBusModemmanagerService, DBusModemmanagerPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to service %s", DBusModemmanagerService)
	}

	// It may take 30+ seconds for a Modem object to appear after an Inhibit or
	// a reset, so poll the managed objects for 60 seconds looking for a Modem.
	modemPath := dbus.ObjectPath("")
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		managed, err := dbusutil.ManagedObjects(ctx, obj)
		if err != nil {
			return errors.Wrap(err, "failed to get ManagedObjects")
		}
		for iface, paths := range managed {
			if iface == DBusModemmanagerModemInterface {
				if len(paths) > 0 {
					modemPath = paths[0]
				}
				break
			}
		}
		if modemPath == dbus.ObjectPath("") {
			return errors.Wrap(err, "failed to get Modem path")
		}
		return nil // success
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return nil, err
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerModemInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// GetSimpleModem creates a PropertyHolder for the SimpleModem object
func (m *Modem) GetSimpleModem(ctx context.Context) (*Modem, error) {
	modemPath := dbus.ObjectPath(m.String())
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerSimpleModemInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// GetSimProperties creates a PropertyHolder for the Sim object and returns the associated Properties.
func (m *Modem) GetSimProperties(ctx context.Context, simPath dbus.ObjectPath) (*dbusutil.Properties, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerSimInterface, simPath)
	if err != nil {
		return nil, err
	}
	return ph.GetProperties(ctx)
}

// GetSimSlots uses the Modem.SimSlots property to fetch SimProperties for each slot.
// Returns the array of SimProperties and the array index of the primary slot on success.
// If a slot path is empty, the entry for that slot will be nil.
func (m *Modem) GetSimSlots(ctx context.Context) (simProps []*dbusutil.Properties, primary uint32, err error) {
	modemProps, err := m.GetProperties(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to call Modem.GetProperties")
	}
	simSlots, err := modemProps.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		return nil, 0, errors.Wrap(err, "missing Modem.SimSlots property")
	}
	primarySlot, err := modemProps.GetUint32(mmconst.ModemPropertyPrimarySimSlot)
	if err != nil {
		return nil, 0, errors.Wrap(err, "missing Modem.PrimarySimSlot property")
	}
	if primarySlot < 1 {
		return nil, 0, errors.Wrap(err, "Modem.PrimarySimSlot < 1")
	}
	primary = primarySlot - 1

	// Gather Properties for each Modem.SimSlots entry.
	for _, simPath := range simSlots {
		var props *dbusutil.Properties
		if simPath != "/" {
			props, err = m.GetSimProperties(ctx, simPath)
			if err != nil {
				return nil, 0, errors.Wrap(err, "failed to call Sim.GetProperties")
			}
		}
		simProps = append(simProps, props)
	}

	return simProps, primary, nil
}

// PollModem polls for a new modem to appear on D-Bus. oldModem is the D-Bus path of the modem that should disappear.
func PollModem(ctx context.Context, oldModem string) (*Modem, error) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newModem, err := NewModem(ctx)
		if err != nil {
			return err
		}
		if oldModem == newModem.String() {
			return errors.New("old modem still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: mmconst.ModemPollTime}); err != nil {
		return nil, errors.Wrap(err, "modem or its properties not up after switching SIM slot")
	}
	return NewModem(ctx)
}

// NewModemWithSim returns a Modem where the primary SIM slot is not empty.
// Useful on dual SIM DUTs where only one SIM is available, and we want to
// select the slot with the active SIM.
func NewModemWithSim(ctx context.Context) (*Modem, error) {
	modem, err := NewModem(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create modem")
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call GetProperties on modem")
	}
	sim, err := props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return nil, errors.Wrap(err, "missing sim property")
	}
	if sim != mmconst.EmptySlotPath {
		return modem, nil
	}

	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get simslots property")
	}
	for slotIndex, path := range simSlots {
		if path == mmconst.EmptySlotPath {
			continue
		}
		testing.ContextLogf(ctx, "Primary slot doesn't have a SIM, switching to slot %d", slotIndex+1)
		if c := modem.Call(ctx, "SetPrimarySimSlot", uint32(slotIndex+1)); c.Err != nil {
			return nil, errors.Wrap(c.Err, "failed to set primary SIM slot")
		}
		return PollModem(ctx, modem.String())
	}
	return nil, errors.New("failed to create modem: modemmanager D-Bus object has no valid SIM's")
}

// IsEnabled checks modem state and returns boolean
func (m *Modem) IsEnabled(ctx context.Context) (bool, error) {
	props, err := m.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to call GetProperties on modem")
	}
	modemState, err := props.GetInt32(mmconst.ModemPropertyState)
	if err != nil {
		return false, errors.Wrap(err, "missing state property")
	}
	testing.ContextLogf(ctx, "modemState in IsEnabled is %d", modemState)
	states := [6]mmconst.ModemState{
		mmconst.ModemStateEnabled,
		mmconst.ModemStateSearching,
		mmconst.ModemStateRegistered,
		mmconst.ModemStateDisconnecting,
		mmconst.ModemStateConnecting,
		mmconst.ModemStateConnected}

	for _, value := range states {
		if int32(value) == modemState {
			return true, nil
		}
	}
	return false, nil
}

// IsDisabled checks modem state and returns boolean
func (m *Modem) IsDisabled(ctx context.Context) (bool, error) {
	props, err := m.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to call GetProperties on modem")
	}
	modemState, err := props.GetInt32(mmconst.ModemPropertyState)
	if err != nil {
		return false, errors.Wrap(err, "missing state property")
	}
	testing.ContextLogf(ctx, "modemState in IsDisabled is %d", modemState)
	return (modemState == int32(mmconst.ModemStateDisabled)), nil
}

// IsPowered checks modem powerstate and returns true if powered on
func (m *Modem) IsPowered(ctx context.Context) (bool, error) {
	props, err := m.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to call GetProperties on modem")
	}
	modemState, err := props.GetUint32(mmconst.ModemPropertyPowered)
	if err != nil {
		return false, errors.Wrap(err, "missing powerstate property")
	}
	testing.ContextLogf(ctx, "modemState in IsPowered is %d", modemState)
	if modemState == uint32(mmconst.ModemPowerStateOn) {
		return true, nil
	}
	return false, nil
}

// IsRegistered checks modem registration state and returns boolean
func (m *Modem) IsRegistered(ctx context.Context) (bool, error) {
	// for SimpleModem GetStatus returned properties
	var props map[string]interface{}

	if err := m.Call(ctx, "GetStatus").Store(&props); err != nil {
		return false, errors.Wrapf(err, "failed getting properties of %v", m)
	}
	simpleProps := dbusutil.NewProperties(props)
	modemState, err := simpleProps.GetUint32(mmconst.SimpleModemPropertyRegState)
	testing.ContextLogf(ctx, "simple modem regstate is %d", modemState)
	if err != nil {
		return false, errors.Wrap(err, "missing 3gpp reg state property")
	}
	states := [2]mmconst.ModemRegState{
		mmconst.ModemRegStateHome,
		mmconst.ModemRegStateRoaming}

	for _, value := range states {
		if uint32(value) == modemState {
			return true, nil
		}
	}
	return false, nil
}

// IsConnected checks modem state and returns boolean
func (m *Modem) IsConnected(ctx context.Context) (bool, error) {
	// for SimpleModem GetStatus returned properties
	var props map[string]interface{}

	if err := m.Call(ctx, "GetStatus").Store(&props); err != nil {
		return false, errors.Wrapf(err, "failed getting properties of %v", m)
	}
	simpleProps := dbusutil.NewProperties(props)
	modemState, err := simpleProps.GetUint32(mmconst.SimpleModemPropertyState)
	testing.ContextLogf(ctx, "simple modem state is %d", modemState)
	if err != nil {
		return false, errors.Wrap(err, "missing state property")
	}
	states := [2]mmconst.ModemState{
		mmconst.ModemStateConnecting,
		mmconst.ModemStateConnected}

	for _, value := range states {
		if uint32(value) == modemState {
			return true, nil
		}
	}
	return false, nil
}

// EnsureEnabled polls for modem state property to be enabled
func EnsureEnabled(ctx context.Context, modem *Modem) error {
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.New("failed to read modem powered state")
	}
	if !isPowered {
		return errors.New("modem not powered")
	}

	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isEnabled, err := modem.IsEnabled(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch enabled state")
		}
		if !isEnabled {
			return errors.New("modem not enabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to verify modem enabled")
	}
	return nil
}

// EnsureDisabled polls for modem state property to be disabled
func EnsureDisabled(ctx context.Context, modem *Modem) error {
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read modem powered state")
	}
	if isPowered {
		return errors.New("modem still not in low powered state")
	}

	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isEnabled, err := modem.IsEnabled(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch enabled state")
		}
		isDisabled, err := modem.IsDisabled(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch disabled state")
		}
		if isEnabled || !isDisabled {
			return errors.New("still modem not disabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to verify modem disabled")
	}
	return nil
}

// EnsureConnectState polls for modem state to be connected or disconnected
func EnsureConnectState(ctx context.Context, modem, simpleModem *Modem, expectedConnected bool) error {
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		EnsureEnabled(ctx, modem)
		isConnected, err := simpleModem.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch connected state")
		}
		if expectedConnected != isConnected {
			return errors.Errorf("unexpected connect state, got %t, expected %t", isConnected, expectedConnected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to set modem connect state")
	}
	return nil
}

// EnsureRegistered polls for simple modem property m3gpp-registration-state
func EnsureRegistered(ctx context.Context, modem, simpleModem *Modem) error {
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.New("failed to read modem powered state")
	}
	if !isPowered {
		return errors.New("modem not powered")
	}
	isEnabled, err := modem.IsEnabled(ctx)
	if err != nil {
		return errors.New("failed to read modem enabled state")
	}
	if !isEnabled {
		return errors.New("modem not enabled")
	}
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isRegistered, err := simpleModem.IsRegistered(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch reigstration state")
		}
		if !isRegistered {
			return errors.New("modem not registered")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to verify modem registration state")
	}
	return nil
}

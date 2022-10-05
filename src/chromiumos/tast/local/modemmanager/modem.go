// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

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

// GetSimpleModem creates a PropertyHolder for the SimpleModem object.
func (m *Modem) GetSimpleModem(ctx context.Context) (*Modem, error) {
	modemPath := dbus.ObjectPath(m.String())
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerSimpleModemInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// EnableSAR - enable/disable MM SAR
func (m *Modem) EnableSAR(ctx context.Context, enable bool) error {
	err := m.Call(ctx, mmconst.ModemSAREnable, enable).Err
	if err != nil {
		return errors.Wrap(err, "failed to enable/disable SAR")
	}
	return nil
}

// GetSARInterface creates a PropertyHolder for the SAR object.
func (m *Modem) GetSARInterface(ctx context.Context) (*Modem, error) {
	modemPath := dbus.ObjectPath(m.String())
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerSARInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// GetMessagingInterface creates a PropertyHolder for the SAR object.
func (m *Modem) GetMessagingInterface(ctx context.Context) (*Modem, error) {
	modemPath := dbus.ObjectPath(m.String())
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerMessageInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// GetMessagesList Retrieve all SMS messages.
func (m *Modem) GetMessagesList(ctx context.Context) ([]dbus.ObjectPath, error) {
	msgProps, err := m.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call Messages.GetProperties")
	}
	msgList, err := msgProps.GetObjectPaths(mmconst.ModemPropertyMessages)
	if err != nil {
		return nil, errors.Wrap(err, "missing Messages.SimSlots property")
	}

	return msgList, nil
}

// DeleteMessage delete an sms message specified by path.
func (m *Modem) DeleteMessage(ctx context.Context, path dbus.ObjectPath) error {
	err := m.Call(ctx, mmconst.MessagesDelete, path).Err
	if err != nil {
		return errors.Wrapf(err, "failed to delete message with path: %s", path)
	}
	return nil
}

// DeleteAllMessages delete all sms messages
func (m *Modem) DeleteAllMessages(ctx context.Context) error {
	msgInterface, err := m.GetMessagingInterface(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get messaging interface")
	}
	msgList, err := msgInterface.GetMessagesList(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get list of messages")
	}
	for _, path := range msgList {
		testing.ContextLog(ctx, "path: ", path)
		if err := msgInterface.DeleteMessage(ctx, path); err != nil {
			return errors.Wrap(err, "failed to delete message")
		}
	}
	return nil
}

// IsSAREnabled - checks if SAR is enabled
func (m *Modem) IsSAREnabled(ctx context.Context) (bool, error) {
	sarProps, err := m.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to read SAR properties")
	}

	sarState, err := sarProps.GetBool(mmconst.SARState)
	if err != nil {
		return false, errors.Wrap(err, "failed to read SARState")
	}
	return sarState, nil
}

// GetEquipmentIdentifier - get the identity of the device. This will be the IMEI number.
func (m *Modem) GetEquipmentIdentifier(ctx context.Context) (string, error) {
	modemProps, err := m.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read modem properties")
	}

	imei, err := modemProps.GetString(mmconst.ModemPropertyEquipmentIdentifier)
	if err != nil {
		return "", errors.Wrap(err, "failed to read EquipmentIdentifier")
	}
	return imei, nil
}

// GetModem3gpp creates a PropertyHolder for the Modem3gpp object.
func (m *Modem) GetModem3gpp(ctx context.Context) (*Modem, error) {
	modemPath := dbus.ObjectPath(m.String())
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanager3gppModemInterface, modemPath)
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

// GetBearerProperties creates a PropertyHolder for the Bearer object and returns the associated Properties.
func (m *Modem) GetBearerProperties(ctx context.Context, bearerPath dbus.ObjectPath) (*dbusutil.Properties, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerBearerInterface, bearerPath)
	if err != nil {
		return nil, err
	}
	return ph.GetProperties(ctx)
}

// WaitForState waits for the modem to reach a particular state
func (m *Modem) WaitForState(ctx context.Context, state mmconst.ModemState, timeout time.Duration) error {

	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		props, err := m.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get modem properties")
		}
		s, err := props.GetInt32(mmconst.ModemPropertyState)
		if err != nil {
			return errors.Wrap(err, "failed to get modem state")
		}
		if s != int32(state) {
			return errors.Wrapf(err, "Modem did not reach expected state: got: %d, want: %d", s, state)
		}
		return nil // success
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return err
	}
	return nil
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
	simPath, err := props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return nil, errors.Wrap(err, "missing sim property")
	}
	isValidSim, err := modem.isValidSim(ctx, simPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if sim is valid")
	}
	if isValidSim {
		return modem, nil
	}

	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get simslots property")
	}
	for slotIndex, path := range simSlots {
		isValidSim, err := modem.isValidSim(ctx, path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if sim is valid")
		}
		if !isValidSim {
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

// isValidSim checks if a simPath has a connectable sim card.
func (m *Modem) isValidSim(ctx context.Context, simPath dbus.ObjectPath) (bool, error) {
	if simPath == mmconst.EmptySlotPath {
		return false, nil
	}
	simProps, err := m.GetSimProperties(ctx, simPath)
	if err != nil {
		return false, errors.Wrap(err, "failed to read sim properties")
	}
	ESimStatus, err := simProps.GetUint32(mmconst.SimPropertyESimStatus)
	if err != nil {
		return false, errors.Wrap(err, "failed to get ESIMStatus property")
	}
	return ESimStatus != mmconst.ESimStatusNoProfile, nil
}

// IsEnabled checks modem state and returns a boolean.
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

// IsDisabled checks modem state and returns a boolean.
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

// IsPowered checks modem powerstate and returns a boolean.
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

// IsRegistered checks modem registration state and returns a boolean.
func (m *Modem) IsRegistered(ctx context.Context) (bool, error) {
	// for SimpleModem GetStatus returned properties
	var props map[string]interface{}

	if err := m.Call(ctx, "GetStatus").Store(&props); err != nil {
		return false, errors.Wrapf(err, "failed getting properties of %v", m)
	}
	simpleProps := dbusutil.NewProperties(props)
	modemState, err := simpleProps.GetUint32(mmconst.SimpleModemPropertyRegState)
	testing.ContextLogf(ctx, "SimpleModem regstate is %d", modemState)
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

// IsConnected checks modem state and returns a boolean.
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

// EnsureEnabled polls for modem state property to be enabled.
func EnsureEnabled(ctx context.Context, modem *Modem) error {
	// poll for expected power state as powered state change can take time
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isPowered, err := modem.IsPowered(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read modem powered state")
		}
		if !isPowered {
			return errors.New("still modem not powered")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 200 * time.Millisecond,
	}); err != nil {
		return errors.Wrap(err, "failed to verify modem power state")
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

// EnsureDisabled polls for modem state property to be disabled.
func EnsureDisabled(ctx context.Context, modem *Modem) error {
	// poll for expected power state as powered state change can take time
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isPowered, err := modem.IsPowered(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read modem powered state")
		}
		if isPowered {
			return errors.New("still modem not in low powered state")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 200 * time.Millisecond,
	}); err != nil {
		return errors.Wrap(err, "failed to verify modem power state")
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

// EnsureConnectState polls for modem state to be connected or disconnected.
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

// EnsureRegistered polls for simple modem property m3gpp-registration-state.
func EnsureRegistered(ctx context.Context, modem, simpleModem *Modem) error {
	if isPowered, err := modem.IsPowered(ctx); err != nil {
		return errors.New("failed to read modem powered state")
	} else if !isPowered {
		return errors.New("modem not powered")
	}
	if isEnabled, err := modem.IsEnabled(ctx); err != nil {
		return errors.New("failed to read modem enabled state")
	} else if !isEnabled {
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

// Connect polls on simple modem D-Bus connect call with given apn.
func Connect(ctx context.Context, modem *Modem, props map[string]interface{}, timeout time.Duration) error {
	// Connect and poll for modem state.
	return testing.Poll(ctx, func(ctx context.Context) error {
		errConn := modem.Call(ctx, mmconst.ModemConnect, props).Err
		if (errConn != nil) && (strings.Contains(errConn.Error(), "no-service")) {
			return errors.Wrap(errConn, "failed to connect can be network issue")
		}
		if isConnected, err := modem.IsConnected(ctx); err != nil {
			return errors.Wrap(err, "failed to fetch connected state")
		} else if !isConnected {
			return errors.Wrap(err, "modem not connected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: 2 * time.Second,
	})
}

// InhibitModem inhibits the first available modem on DBus. Use the returned callback to uninhibit.
func InhibitModem(ctx context.Context) (func(ctx context.Context) error, error) {
	modem, err := NewModem(ctx)
	emptyUninhibit := func(ctx context.Context) error { return nil }
	if err != nil {
		return emptyUninhibit, errors.Wrap(err, "failed to create Modem")
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		return emptyUninhibit, errors.Wrap(err, "failed to call GetProperties on Modem")
	}
	device, err := props.GetString(mmconst.ModemPropertyDevice)
	if err != nil {
		return emptyUninhibit, errors.Wrap(err, "missing Device property")
	}

	obj, err := dbusutil.NewDBusObject(ctx, DBusModemmanagerService, DBusModemmanagerInterface, DBusModemmanagerPath)
	if err != nil {
		return emptyUninhibit, errors.Wrap(err, "unable to connect to ModemManager1")
	}
	if err := obj.Call(ctx, "InhibitDevice", device, true).Err; err != nil {
		return emptyUninhibit, errors.Wrap(err, "inhibitDevice(true) failed")
	}

	uninhibit := func(ctx context.Context) error {
		if err := obj.Call(ctx, "InhibitDevice", device, false).Err; err != nil {
			return errors.Wrap(err, "inhibitDevice(false) failed")
		}
		return nil
	}
	return uninhibit, nil
}

// SetPrimarySimSlot switches primary SIM slot to a given slot
func SetPrimarySimSlot(ctx context.Context, primary uint32) error {
	modem, err := NewModem(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create modem")
	}
	if c := modem.Call(ctx, "SetPrimarySimSlot", primary); c.Err != nil {
		return errors.Wrapf(c.Err, "failed while switching the primary sim slot to: %d", primary)
	}
	if _, err = PollModem(ctx, modem.String()); err != nil {
		return errors.Wrapf(err, "could not find modem after switching the primary slot to: %d", primary)
	}
	return nil
}

// SwitchSlot switches from current slot to new slot on dual sim duts, returns new primary slot.
func SwitchSlot(ctx context.Context) (uint32, error) {

	modem, err := NewModem(ctx)
	if err != nil {
		return math.MaxUint32, errors.Wrap(err, "failed to create modem")
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		return math.MaxUint32, errors.Wrap(err, "failed to call getproperties on modem")
	}
	_, err = props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return math.MaxUint32, errors.Wrap(err, "missing sim property")
	}
	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		return math.MaxUint32, errors.Wrap(err, "failed to get simslots property")
	}
	primary, err := props.GetUint32(mmconst.ModemPropertyPrimarySimSlot)
	if err != nil {
		return math.MaxUint32, errors.Wrap(err, "missing primarysimslot property")
	}
	numSlots := len(simSlots)
	testing.ContextLogf(ctx, "Number of slots on dut: %d", numSlots)
	if numSlots < 2 {
		return primary, nil
	}

	// For dual sim devices
	var setSlot uint32 = 1
	if primary == setSlot {
		setSlot = 2
	}
	if err := SetPrimarySimSlot(ctx, setSlot); err != nil {
		return math.MaxUint32, err
	}

	return uint32(setSlot), nil
}

// GetSimProperty gets current modem sim property.
func (m *Modem) GetSimProperty(ctx context.Context, propertyName string) (string, error) {
	modemProps, err := m.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to call getproperties on modem")
	}
	simPath, err := modemProps.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return "", errors.Wrap(err, "failed to get modem sim property")
	}
	testing.ContextLogf(ctx, "SIM path =%s", simPath)
	simProps, err := m.GetSimProperties(ctx, simPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read sim properties")
	}
	info, err := simProps.GetString(propertyName)
	if err != nil {
		return "", errors.Wrapf(err, "error getting property %q", propertyName)
	}

	return info, nil
}

// GetEid gets current modem sim eid, return eid if esim is active.
func (m *Modem) GetEid(ctx context.Context) (string, error) {
	return m.GetSimProperty(ctx, mmconst.SimPropertySimEid)
}

// GetIMSI gets current modem sim IMSI, return IMSI if sim is active.
func (m *Modem) GetIMSI(ctx context.Context) (string, error) {
	return m.GetSimProperty(ctx, mmconst.SimPropertySimIMSI)
}

// GetOperatorIdentifier gets current modem sim Operator Identifier, return Operator Identifier if sim is active.
func (m *Modem) GetOperatorIdentifier(ctx context.Context) (string, error) {
	return m.GetSimProperty(ctx, mmconst.SimPropertySimOperatorIdentifier)
}

// GetSimIdentifier gets current modem sim Identifier, return Identifier if sim is active.
func (m *Modem) GetSimIdentifier(ctx context.Context) (string, error) {
	return m.GetSimProperty(ctx, mmconst.SimPropertySimIdentifier)
}

// GetOperatorCode gets current operator code, return operator code if sim is active.
func (m *Modem) GetOperatorCode(ctx context.Context) (string, error) {
	modem3gpp, err := m.GetModem3gpp(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get 3gpp modem")
	}
	modemProps, err := modem3gpp.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to call getproperties on modem3gpp")
	}
	operatorCode, err := modemProps.GetString(mmconst.ModemModem3gppPropertyOperatorCode)
	if err != nil {
		return "", errors.Wrapf(err, "error getting property %q", mmconst.ModemModem3gppPropertyOperatorCode)
	}

	return operatorCode, nil
}

// SetInitialEpsBearerSettings sets the Attach APN.
func SetInitialEpsBearerSettings(ctx context.Context, modem3gpp *Modem, props map[string]interface{}) error {
	if c := modem3gpp.Call(ctx, "SetInitialEpsBearerSettings", props); c.Err != nil {
		return errors.Wrap(c.Err, "failed to set initial EPS bearer settings")
	}
	return nil
}

// GetInitialEpsBearerSettings gets the Attach APN.
func (m *Modem) GetInitialEpsBearerSettings(ctx context.Context, modem *Modem) (map[string]interface{}, error) {
	modemPath := ObjectPath{dbus.ObjectPath(m.String()), nil}
	apnPropsGet := modemPath.GetPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanager3gppModemInterface).
		GetProperties(ctx).
		GetObjectPath(mmconst.ModemModem3gppPropertyInitialEpsBearer).
		GetObjectProperties(ctx, DBusModemmanagerService, DBusModemmanagerBearerInterface).
		Get(mmconst.BearerPropertyProperties)

	if apnPropsGet.err != nil {
		return nil, errors.Wrap(apnPropsGet.err, "failed to read InitialEpsBearer properties")
	}
	bearerProps, ok := apnPropsGet.iface.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to parse InitialEpsBearer properties")
	}
	return bearerProps, nil
}

// GetFirstConnectedBearer gets the apn information of the first connected bearer.
func (m *Modem) GetFirstConnectedBearer(ctx context.Context, modem *Modem) (map[string]interface{}, error) {
	modemPath := ObjectPath{dbus.ObjectPath(m.String()), nil}
	bearerPaths := modemPath.GetPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerModemInterface).
		GetProperties(ctx).
		GetObjectPaths(mmconst.ModemPropertyBearers)

	if bearerPaths.err != nil {
		return nil, errors.Wrap(bearerPaths.err, "failed to read Bearer paths")
	}
	for _, opath := range bearerPaths.objectPaths {
		bearerPath := ObjectPath{opath, nil}
		bearerProps := bearerPath.GetPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerBearerInterface).
			GetProperties(ctx)
		if bearerProps.err != nil {
			return nil, errors.Wrap(bearerProps.err, "failed to read bearer properties")
		}
		connected, err := bearerProps.properties.GetBool(mmconst.BearerPropertyConnected)
		if err != nil {
			return nil, errors.Wrap(err, "missing connected property")
		}
		if connected == true {
			apnPropsGet, err := bearerProps.properties.Get(mmconst.BearerPropertyProperties)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read bearer properties")
			}
			bearerProps, ok := apnPropsGet.(map[string]interface{})
			if !ok {
				return nil, errors.New("failed to parse bearer properties")
			}
			return bearerProps, nil
		}
	}
	return nil, nil
}

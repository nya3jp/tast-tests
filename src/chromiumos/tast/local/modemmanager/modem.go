// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"
	"math"
	"strings"
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

// GetSimpleModem creates a PropertyHolder for the SimpleModem object.
func (m *Modem) GetSimpleModem(ctx context.Context) (*Modem, error) {
	modemPath := dbus.ObjectPath(m.String())
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerSimpleModemInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
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
	if isPowered, err := modem.IsPowered(ctx); err != nil {
		return errors.New("failed to read modem powered state")
	} else if !isPowered {
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

// EnsureDisabled polls for modem state property to be disabled.
func EnsureDisabled(ctx context.Context, modem *Modem) error {
	if isPowered, err := modem.IsPowered(ctx); err != nil {
		return errors.Wrap(err, "failed to read modem powered state")
	} else if isPowered {
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

// GetEid gets current modem sim eid, return eid if esim is active.
func (m *Modem) GetEid(ctx context.Context) (string, error) {
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
	simEid, err := simProps.GetString(mmconst.SimPropertySimEid)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sim eid property")
	}

	return simEid, nil
}

// GetActiveSimPuk gets puk for psim iccid, sets primary slot to psim.
func (m *Modem) GetActiveSimPuk(ctx context.Context) (string, error) {

	if simEid, err := m.GetEid(ctx); err != nil {
		return "", errors.Wrap(err, "failed to get sim eid property")
	} else if simEid != "" {
		// Switch slot to get to active psim.
		if _, err := SwitchSlot(ctx); err != nil {
			return "", errors.Wrap(err, "could not get puk")
		}
	}

	m, err := NewModem(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get modem after slot switch")
	}

	if simEid, err := m.GetEid(ctx); err != nil {
		return "", errors.Wrap(err, "failed to get sim eid property")
	} else if simEid != "" {
		return "", errors.New("could not get active psim to get known puks")
	}

	// Get ICCID to get puk.
	modemProps, err := m.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to call getproperties on modem")
	}

	simPath, err := modemProps.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		return "", errors.Wrap(err, "failed to get modem sim property")
	}

	simProps, err := m.GetSimProperties(ctx, simPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read sim properties")
	}

	simICCID, err := simProps.GetString(mmconst.SimPropertySimIdentifier)
	if err != nil {
		return "", errors.Wrap(err, "failed to get sim simIdentifier property")
	}

	puk, err := GetPuk(ctx, simICCID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get puk code")
	}

	return puk, nil
}

// GetPuk gets puk code for given sim iccid from known iccid, puk pairs/list.
func GetPuk(ctx context.Context, iccid string) (string, error) {
	// Map of ICCID and corresponding PUK codes in multisim dut pool.
	pukCodes := map[string]string{
		"8901260153779127425":  "67308773",
		"8901260153779127706":  "31224207",
		"89012804320036371960": "44604241", // ATT
		"89148000004796350513": "47183865"} // Verizon

	pukVal := ""
	if _, ok := pukCodes[iccid]; ok {
		pukVal = pukCodes[iccid]
	}

	return pukVal, nil
}

// SetInitialEpsBearerSettings sets the Attach APN.
func SetInitialEpsBearerSettings(ctx context.Context, modem3gpp *Modem, props map[string]interface{}) error {
	if c := modem3gpp.Call(ctx, "SetInitialEpsBearerSettings", props); c.Err != nil {
		return errors.Wrap(c.Err, "failed to set initial EPS bearer settings")
	}
	return nil
}

// // GetInitialEpsBearerSettings sets the Attach APN.
// func GetInitialEpsBearerSettings(ctx context.Context, modem *Modem) (map[string]interface{}, error) {

// 	modem3gpp, err := modem.GetModem3gpp(ctx)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "could not get modem3gpp object")
// 	}
// 	modemProps, err := modem3gpp.GetProperties(ctx)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to call getproperties on modem")
// 	}
// 	bearerPath, err := modemProps.GetObjectPath(mmconst.ModemPropertyInitialEpsBearer)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to get modem bearer property")
// 	}
// 	bearerProps, err := modem.GetBearerProperties(ctx, bearerPath)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to read bearer properties")
// 	}
// 	apnProps, err := bearerProps.Get("Properties") //TODO: create const?
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to read InitialEpsBearer properties")
// 	}
// 	testing.ContextLog(ctx, "apnProps", apnProps) //TODO:remove
// 	return apnProps.(map[string]interface{}), nil
// }

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

// GetInitialEpsBearerSettings sets the Attach APN.
func (m *Modem) GetInitialEpsBearerSettings(ctx context.Context, modem *Modem) (map[string]interface{}, error) {
	eModemPath := eObjectPath{dbus.ObjectPath(m.String()), nil}
	apnPropsGet := eModemPath.gGetPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanager3gppModemInterface).
		gGetProperties(ctx).
		gGetObjectPath(mmconst.ModemPropertyInitialEpsBearer).
		gGetObjectProperties(ctx, DBusModemmanagerService, DBusModemmanagerBearerInterface).
		gGet("Properties")

	//TODO: create const for `Properties`?
	if apnPropsGet.err != nil {
		return nil, errors.Wrap(apnPropsGet.err, "failed to read InitialEpsBearer properties")
	}
	testing.ContextLog(ctx, "apnProps", apnPropsGet.iface) //TODO:remove
	return apnPropsGet.iface.(map[string]interface{}), nil
}

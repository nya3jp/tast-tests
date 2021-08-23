// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mmconst defines the constants for ModemManager1,
// This is defined under common/ as they might be used in both
// local and remote tests.
package mmconst

import (
	"time"
)

// ModemManager1.Modem properties
const (
	ModemPropertyDevice         = "Device"
	ModemPropertySim            = "Sim"
	ModemPropertySimSlots       = "SimSlots"
	ModemPropertyPrimarySimSlot = "PrimarySimSlot"
	ModemPropertyState          = "State"
	ModemPropertyPowered        = "PowerState"
)

// ModemManager1.Modem.Simple properties
const (
	SimpleModemPropertyState    = "state"
	SimpleModemPropertyRegState = "m3gpp-registration-state"
)

// ModemManager1.Sim properties
const (
	SimPropertySimIdentifier = "SimIdentifier"
)

// Wait times for modem at Modemmanager operations
const (
	ModemPollTime = 1 * time.Minute
)

// D-Bus path for empty sim slots
const (
	EmptySlotPath = "/"
)

// ModemState states from Modemmanager-enums.h
type ModemState int32

// All the modem states
const (
	ModemStateFailed        ModemState = -1
	ModemStateUnknown       ModemState = 0
	ModemStateInitializing  ModemState = 1
	ModemStateLocked        ModemState = 2
	ModemStateDisabled      ModemState = 3
	ModemStateDisabling     ModemState = 4
	ModemStateEnabling      ModemState = 5
	ModemStateEnabled       ModemState = 6
	ModemStateSearching     ModemState = 7
	ModemStateRegistered    ModemState = 8
	ModemStateDisconnecting ModemState = 9
	ModemStateConnecting    ModemState = 10
	ModemStateConnected     ModemState = 11
)

// ModemRegState for registration states
type ModemRegState uint32

// All the 3gpp registration states
const (
	ModemRegStateIdle      ModemRegState = 0
	ModemRegStateHome      ModemRegState = 1
	ModemRegStateSearching ModemRegState = 2
	ModemRegStateDenied    ModemRegState = 3
	ModemRegStateUnknown   ModemRegState = 4
	ModemRegStateRoaming   ModemRegState = 5
)

// ModemPowerState is states of MMModemPowerState
type ModemPowerState uint32

// All the modem power states
const (
	ModemPowerStateUnknown ModemPowerState = 0
	ModemPowerStateOff     ModemPowerState = 1
	ModemPowerStateLow     ModemPowerState = 2
	ModemPowerStateOn      ModemPowerState = 3
)

// Modem DBus methods
const (
	// Modem interface methods
	ModemEnable = "Enable"

	// Modem.Simple interface methods
	ModemConnect    = "Connect"
	ModemDisconnect = "Disconnect"
)

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

// Modem States from Modemmanager-enums.h
type ModemState int32

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

// MMModemPowerState
type ModemPowerState int

const (
	ModemPowerStateUnknown ModemPowerState = 0
	ModemPowerStateOff     ModemPowerState = 1
	ModemPowerStateLow     ModemPowerState = 2
	ModemPowerStateOn      ModemPowerState = 3
)

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mmconst defines the constants for ModemManager1,
// This is defined under common/ as they might be used in both
// local and remote tests.
package mmconst

import "time"

// ModemManager1.Modem properties
const (
	ModemPropertyDevice         = "Device"
	ModemPropertySim            = "Sim"
	ModemPropertySimSlots       = "SimSlots"
	ModemPropertyPrimarySimSlot = "PrimarySimSlot"
	ModemPropertyState          = "State"
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

// States that a modem DBus object can be in
const (
	ModemStateFailed        = -1
	ModemStateUnknown       = 0
	ModemStateInitializing  = 1
	ModemStateLocked        = 2
	ModemStateDisabled      = 3
	ModemStateDisabling     = 4
	ModemStateEnabling      = 5
	ModemStateEnabled       = 6
	ModemStateSearching     = 7
	ModemStateRegistered    = 8
	ModemStateDisconnecting = 9
	ModemStateConnecting    = 10
	ModemStateConnected     = 11
)

// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mmconst defines the constants for ModemManager1,
// This is defined under common/ as they might be used in both
// local and remote tests.
package mmconst

import "time"

// ModemManager1.Modem properties
const (
	ModemPropertyBearers             = "Bearers"
	ModemPropertyDevice              = "Device"
	ModemPropertyEquipmentIdentifier = "EquipmentIdentifier"
	ModemPropertyManufacturer        = "Manufacturer"
	ModemPropertyOwnNumbers          = "OwnNumbers"
	ModemPropertyPowered             = "PowerState"
	ModemPropertyPrimarySimSlot      = "PrimarySimSlot"
	ModemPropertySim                 = "Sim"
	ModemPropertySimSlots            = "SimSlots"
	ModemPropertyState               = "State"
)

// ModemManager1.Modem.Modem3gpp properties
const (
	ModemModem3gppPropertyInitialEpsBearer = "InitialEpsBearer"
	ModemModem3gppPropertyOperatorCode     = "OperatorCode"
)

// ModemManager1.Modem.Simple properties
const (
	SimpleModemPropertyState    = "state"
	SimpleModemPropertyRegState = "m3gpp-registration-state"
)

// ModemManager1.Modem.Signal properties
const (
	SignalPropertyLte     = "Lte"
	SignalPropertyLteRsrp = "rsrp"
	SignalPropertyLteRsrq = "rsrq"
	SignalPropertyLteSnr  = "snr"
)

// ModemManager1.Sim properties
const (
	SimPropertySimIMSI               = "Imsi"
	SimPropertySimIdentifier         = "SimIdentifier"
	SimPropertySimEid                = "Eid"
	SimPropertyESimStatus            = "EsimStatus"
	SimPropertySimOperatorIdentifier = "OperatorIdentifier"
)

// ModemManager1.Bearer properties
const (
	BearerPropertyConnected  = "Connected"
	BearerPropertyProperties = "Properties"
)

// BearerIPFamily IP families from Modemmanager-enums.h
type BearerIPFamily uint32

// All the bearer IP families
const (
	BearerIPFamilyNone   BearerIPFamily = 0
	BearerIPFamilyIPv4   BearerIPFamily = 1 << 0
	BearerIPFamilyIPv6   BearerIPFamily = 1 << 1
	BearerIPFamilyIPv4v6 BearerIPFamily = 1 << 2
	BearerIPFamilyAny    BearerIPFamily = 0xFFFFFFFF
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

// Modem Sar DBus methods
const (
	ModemSAREnable = "Enable"
	SARState       = "State"
	SARPowerLevel  = "PowerLevel"
)

// Default SIM pin
const (
	DefaultSimPin = "1111"
	TempSimPin    = "1600"
)

// Possible values for ESimStatus
const (
	ESimStatusUnknown      = 0
	ESimStatusNoProfile    = 1
	ESimStatusWithProfiles = 2
)

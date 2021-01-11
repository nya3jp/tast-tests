// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mmconst defines the constants for ModemManager1,
// This is defined under common/ as they might be used in both
// local and remote tests.
package mmconst

// ModemManager1.Modem properties
const (
	ModemPropertySim            = "Sim"
	ModemPropertySimSlots       = "SimSlots"
	ModemPropertyPrimarySimSlot = "PrimarySimSlot"
)

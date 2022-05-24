// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file defines constants used by both local and remote firmware tests.
*/

// BootMode is a string representing the DUT's firmware boot-mode.
// It is intended to be used with the constants defined below.
type BootMode string

// DUTs have four possible boot modes: Normal, Dev, USBDev, and Recovery.
const (
	BootModeNormal      BootMode = "normal"
	BootModeDev         BootMode = "dev"
	BootModeUSBDev      BootMode = "usbdev"
	BootModeRecovery    BootMode = "rec"
	BootModeUnspecified BootMode = "unspecified"
)

// RWSection refers to one of the two RW firmware sections installed on a Chromebook.
type RWSection string

// There are two RW sections of firmware: "A" and "B". Normally, A is used unless B is required, such as while updating A.
const (
	RWSectionA           RWSection = "A"
	RWSectionB           RWSection = "B"
	RWSectionUnspecified RWSection = ""
)

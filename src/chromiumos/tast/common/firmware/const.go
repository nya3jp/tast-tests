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

// DUTs have three possible boot modes: Normal, Dev, and Recovery.
const (
	BootModeNormal      BootMode = "normal"
	BootModeDev         BootMode = "dev"
	BootModeRecovery    BootMode = "rec"
	BootModeUnspecified BootMode = "unspecified"
)

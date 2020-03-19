// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file defines constants used by both local and remote firmware tests.
*/

import (
	fwpb "chromiumos/tast/services/cros/firmware"
)

// BootMode is a string representing the DUT's firmware boot-mode.
// It is intended to be used with the constants defined below.
type BootMode string

// DUTs have three possible boot modes: Normal, Dev, and Recovery.
const (
	BootModeNormal   BootMode = "normal"
	BootModeDev      BootMode = "dev"
	BootModeRecovery BootMode = "rec"
)

// ProtoBootMode maps the BootMode values to their fwpb equivalents.
var ProtoBootMode = map[BootMode]fwpb.BootMode{
	BootModeNormal:   fwpb.BootMode_BOOT_MODE_NORMAL,
	BootModeDev:      fwpb.BootMode_BOOT_MODE_DEV,
	BootModeRecovery: fwpb.BootMode_BOOT_MODE_RECOVERY,
}

// BootModeFromProto maps the fwpb BootMode values to their common equivalents.
var BootModeFromProto = map[fwpb.BootMode]BootMode{
	fwpb.BootMode_BOOT_MODE_NORMAL:   BootModeNormal,
	fwpb.BootMode_BOOT_MODE_DEV:      BootModeDev,
	fwpb.BootMode_BOOT_MODE_RECOVERY: BootModeRecovery,
}

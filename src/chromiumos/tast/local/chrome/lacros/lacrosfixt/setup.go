// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

import "fmt"

// SetupMode describes how lacros-chrome should be set-up during the test.
// See the SetupMode constants for more explanation. Use Rootfs as a default.
// Note that if the lacrosDeployedBinary var is specified, the lacros binary
// located at the path specified by that var will be used in all cases.
type SetupMode int

const (
	// External denotes a lacros-chrome downloaded per the external data dependency.
	// This mode for downloadable lacros-chrome is deprecated and should not be explicitly used.
	External SetupMode = iota
	// Omaha is used to get the lacros binary.
	Omaha
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
	// For tests that don't care which lacros they are using, use this as a default.
	Rootfs
)

// String returns string representation of the SetupMode.
func (s SetupMode) String() string {
	switch s {
	case External:
		return "External"
	case Omaha:
		return "Omaha"
	case Rootfs:
		return "Rootfs"
	default:
		return fmt.Sprintf("Unknown SetupMode(%d)", s)
	}
}

// LacrosAvailability describes whether Lacros is enabled as a primary browser or else.
type LacrosAvailability string

// Valid values for LacrosAvailability.
const (
	LacrosPrimary LacrosAvailability = "LacrosPrimary"
	LacrosOnly    LacrosAvailability = "LacrosOnly"
	NotSpecified  LacrosAvailability = "NotSpecified"
)

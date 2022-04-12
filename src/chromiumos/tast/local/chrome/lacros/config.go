// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

// UserDataDir is the directory that contains the user data of lacros.
const UserDataDir = "/home/chronos/user/lacros/"

// Selection describes how lacros-chrome should be set-up during the test.
// See the Selection constants for more explanation. Use Rootfs as a default.
// Note that if the lacrosDeployedBinary var is specified, the lacros binary
// located at the path specified by that var will be used in all cases.
type Selection string

const (
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
	// For tests that don't care which lacros they are using, use this as a default.
	Rootfs Selection = "Rootfs"
	// Omaha is used to get the lacros binary.
	Omaha Selection = "Omaha"
)

// Mode describes whether Lacros is enabled as a primary browser or else.
type Mode string

// Valid values for Mode.
const (
	LacrosSideBySide Mode = "LacrosSideBySide"
	LacrosPrimary    Mode = "LacrosPrimary"
	LacrosOnly       Mode = "LacrosOnly"
	NotSpecified     Mode = "NotSpecified"
)

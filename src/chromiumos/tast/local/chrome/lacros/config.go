// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	internal "chromiumos/tast/local/chrome/internal/lacros"
	"chromiumos/tast/testing"
)

const (
	// UserDataDir is the directory that contains the user data of lacros.
	UserDataDir = internal.UserDataDir

	// LacrosSquashFSPath indicates the location of the rootfs lacros squashfs filesystem.
	LacrosSquashFSPath = "/opt/google/lacros/lacros.squash"
)

// DeployedBinary describes the location of a lacros binary that has been
// separately deployed to the device. For example, /usr/local/lacros-chrome.
// This is useful to run tests in the Chromium CI with newer version of lacros,
// or for developers to test against their own local builds of lacros.
// If this is empty, the lacros described by lacros.Selection is used.
var DeployedBinary = testing.RegisterVarString(
	"lacros.DeployedBinary",
	"",
	"The location of a lacros binary that has been separately deployed to the device. Example: --var=lacros.DeployedBinary=/usr/local/lacros-chrome",
)

// Selection describes how lacros-chrome should be set-up during the test.
// See the Selection constants for more explanation. Use Rootfs as a default.
// Note that if the lacros.DeployedBinary var is specified, the lacros binary
// located at the path specified by that var will be used in all cases.
type Selection string

const (
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
	// For tests that don't care which lacros they are using, use this as a default.
	Rootfs Selection = "Rootfs"
	// Omaha is used to get the lacros binary.
	Omaha Selection = "Omaha"
	// NotSelected is used for tests that don't need to specify what lacros to select. eg, AutoUpdate that verifies the selection logic itself.
	NotSelected Selection = "NotSelected"
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

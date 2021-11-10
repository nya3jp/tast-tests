// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

// Constants used by both local/remote tests/services for Lacros.

// For common directories or paths
const (
	// LacrosUserDataDir is the directory that contains the user data of Lacros.
	LacrosUserDataDir       = "/home/chronos/user/lacros/"
	LacrosRootComponentPath = "/home/chronos/cros-components/"
	LacrosFailLogDir        = "/tmp/lacros_faillog/"
)

// For Lacros update tests and services
const (
	CorruptStatefulFilePath  = "/mnt/stateful_partition/.corrupt_stateful"
	RootfsLacrosImageFileURL = "file:///opt/google/lacros"
	BogusComponentUpdaterURL = "http://localhost:12345"
	VersionURL               = "chrome://version/"

	// Lacros component names.
	LacrosCanaryChannelName = "lacros-dogfood-canary"
	LacrosDevChannelName    = "lacros-dogfood-dev"
	LacrosBetaChannelName   = "lacros-dogfood-beta"
	LacrosStableChannelName = "lacros-dogfood-stable"
)

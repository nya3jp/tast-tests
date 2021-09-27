// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

// Constants used by Lacros update remote test or test service.
const (
	LacrosRootComponentPath  = "/home/chronos/cros-components/"
	CorruptStatefulFilePath  = "/mnt/stateful_partition/.corrupt_stateful"
	RootfsLacrosImageFileURL = "file:///opt/google/lacros"
	BogusComponentUpdaterURL = "http://localhost:12345"
	VersionURL               = "chrome://version/"
)

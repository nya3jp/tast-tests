// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ashproc provides utilities to find ash Chrome (a.k.a. chromeos-chrome) processes.
package ashproc

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/chrome/internal/chromeproc"
)

// installDir is the path to the directory that contains Chrome executable.
const installDir = "/opt/google/chrome"

// Processes returns ash-chrome processes, including crashpad_handler processes, too.
func Processes() ([]*process.Process, error) {
	return chromeproc.Processes(installDir)
}

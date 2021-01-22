// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/chrome/internal/chromeproc"
)

// ExecPath contains the path to the Chrome executable.
const ExecPath = chromeproc.ExecPath

// GetPIDs returns all PIDs corresponding to Chrome processes (including
// crashpad's handler).
func GetPIDs() ([]int, error) {
	return chromeproc.GetPIDs()
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID() (int, error) {
	return chromeproc.GetRootPID()
}

// GetPluginProcesses returns Chrome plugin processes.
func GetPluginProcesses() ([]process.Process, error) {
	return chromeproc.GetPluginProcesses()
}

// GetRendererProcesses returns Chrome renderer processes.
func GetRendererProcesses() ([]process.Process, error) {
	return chromeproc.GetRendererProcesses()
}

// GetGPUProcesses returns Chrome gpu-process processes.
func GetGPUProcesses() ([]process.Process, error) {
	return chromeproc.GetGPUProcesses()
}

// GetBrokerProcesses returns Chrome broker processes.
func GetBrokerProcesses() ([]process.Process, error) {
	return chromeproc.GetBrokerProcesses()
}

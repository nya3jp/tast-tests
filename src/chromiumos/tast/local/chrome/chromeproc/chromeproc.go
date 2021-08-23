// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeproc provides utilities to find Chrome processes.
package chromeproc

import (
	"context"
	"regexp"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/internal/chromeproc"
)

// Version returns the Chrome browser version. E.g. Chrome version W.X.Y.Z will be reported as a list of strings.
func Version(ctx context.Context) ([]string, error) {
	versionStr, err := testexec.CommandContext(ctx, ashproc.ExecPath, "--version").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chrome version")
	}

	versionRE := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)\.(\d+)`)
	matches := versionRE.FindStringSubmatch(string(versionStr))
	if len(matches) <= 1 {
		return nil, errors.Errorf("can't recognize version string: %s", string(versionStr))
	}
	return matches[1:], nil
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID() (int, error) {
	p, err := chromeproc.Root(ashproc.ExecPath)
	if err != nil {
		return -1, err
	}
	return int(p.Pid), nil
}

// GetPluginProcesses returns Chrome plugin processes.
func GetPluginProcesses() ([]*process.Process, error) {
	return chromeproc.PluginProcesses(ashproc.ExecPath)
}

// GetRendererProcesses returns Chrome renderer processes.
func GetRendererProcesses() ([]*process.Process, error) {
	return chromeproc.RendererProcesses(ashproc.ExecPath)
}

// GetGPUProcesses returns Chrome gpu-process processes.
func GetGPUProcesses() ([]*process.Process, error) {
	return chromeproc.GPUProcesses(ashproc.ExecPath)
}

// GetBrokerProcesses returns Chrome broker processes.
func GetBrokerProcesses() ([]*process.Process, error) {
	return chromeproc.BrokerProcesses(ashproc.ExecPath)
}

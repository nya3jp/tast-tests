// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosproc provides utilities to find lacros Chrome processes.
package lacrosproc

import (
	"io/ioutil"
	"regexp"
	"strconv"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

// FromLogfile reads the lacros PID from chromeLogFile and returns a process with that PID.
// If multiple PIDs are found, it returns the most recent one.
func FromLogfile(chromeLogFile string) (*process.Process, error) {
	b, err := ioutil.ReadFile(chromeLogFile)
	if err != nil {
		return nil, err
	}

	regex := regexp.MustCompile(`Launched lacros-chrome with pid (\d+)`)
	match := regex.FindAllStringSubmatch(string(b), -1)
	if match == nil {
		return nil, errors.New("could not find lacros PID in chrome log")
	}

	// Get the PID of the most recent log entry.
	pid, err := strconv.Atoi(match[len(match)-1][1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed converting %q to int", match[1])
	}

	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create process for pid %q", pid)
	}

	return proc, nil
}

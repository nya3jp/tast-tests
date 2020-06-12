// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"github.com/shirou/gopsutil/process"
)

// processRunning returns if a named process is running.
func processRunning(procName string) (bool, error) {
	ps, err := process.Processes()
	if err != nil {
		return false, err
	}
	for _, p := range ps {
		n, err := p.Name()
		if err != nil {
			continue
		}
		if n == procName {
			return true, nil
		}
	}
	return false, nil
}

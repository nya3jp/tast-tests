// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package systemd contains utilities related to systemd.
package systemd

import (
	"fmt"
	"os"
	"path/filepath"
)

// Enabled returns whether systemd is used on the current system.
func Enabled() (bool, error) {
	// Check whether init process executable is systemd or not.
	p, err := os.Readlink("/proc/1/exe")
	if err != nil {
		return false, fmt.Errorf("Could not read init exe: ", err)
	}
	return filepath.Base(p) == "systemd", nil
}

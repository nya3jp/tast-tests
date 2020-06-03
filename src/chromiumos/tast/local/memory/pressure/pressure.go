// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pressure provides a mechanism for reading ChromeOS's memory pressure
// level.
package pressure

import (
	"io/ioutil"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// readFirstUint reads the first unsigned integer from a file.
func readFirstUint(f string) (uint, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	firstString := strings.Split(strings.TrimSpace(string(data)), " ")[0]
	firstUint, err := strconv.ParseUint(firstString, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return uint(firstUint), nil
}

// Available returns the amount of currently available memory in MB.
func Available() (uint, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	return readFirstUint(availableMemorySysFile)
}

// CriticalMargin returns the available memory threshold below which the system
// is under critical memory pressure.
func CriticalMargin() (uint, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	return readFirstUint(marginMemorySysFile)
}

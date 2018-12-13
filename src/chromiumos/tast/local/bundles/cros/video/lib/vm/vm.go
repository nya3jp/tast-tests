// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vm is a package that provides a function to check if tests are running under QEMU.
package vm

import (
	"io/ioutil"
	"strings"
)

// isVM true if the test is running under QEMU.
var isVM bool

func init() {
	const path = "/sys/devices/virtual/dmi/id/sys_vendor"
	content, err := ioutil.ReadFile(path)

	if err != nil {
		isVM = false
		return
	}

	vendor := strings.TrimSpace(string(content))

	isVM = vendor == "QEMU"
}

// IsRunningOnVM returns true if the test is running under QEMU.
// Please do not use this to skip running your test entirely.
// Instead, introduce a new dependency describing the required feature:
// https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/test_dependencies.md
func IsRunningOnVM() bool {
	return isVM
}

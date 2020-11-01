// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"testing"

	"chromiumos/tast/testing/testcheck"
)

func TestSoftwareDeps(t *testing.T) {
	testcheck.SoftwareDeps(t, testcheck.Glob(t, "printer.*"), []string{"cros_internal", "cups"})
	testcheck.SoftwareDeps(t, testcheck.Glob(t, "printer.Proxy*"), []string{"chrome", "plugin_vm"})
}

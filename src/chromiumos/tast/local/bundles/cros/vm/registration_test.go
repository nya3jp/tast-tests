// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"testing"

	"chromiumos/tast/testing/testcheck"
)

func TestSoftwareDeps(t *testing.T) {
	testcheck.SoftwareDeps(t, testcheck.Glob(t, "vm.*"), []string{"vm_host"})
}

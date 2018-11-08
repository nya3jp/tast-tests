// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	gotesting "testing"

	"chromiumos/tast/testing"
)

const pattern = "printer.*"

func TestSoftwareDeps(t *gotesting.T) {
	testing.CheckSoftwareDeps(t, pattern, []string{"cups"})
}

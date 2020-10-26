// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"chromiumos/tast/dut"
)

// Reporter provides information about the DUT.
type Reporter struct {
	d *dut.DUT
}

// New creates a reporter.
func New(d *dut.DUT) *Reporter {
	return &Reporter{d}
}

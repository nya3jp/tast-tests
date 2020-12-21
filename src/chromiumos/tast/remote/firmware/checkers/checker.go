// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package checkers

import (
	"chromiumos/tast/remote/firmware"
)

// Checker verifies DUT state.
type Checker struct {
	h *firmware.Helper
}

// New creates a Checker.
func New(h *firmware.Helper) *Checker {
	return &Checker{h}
}

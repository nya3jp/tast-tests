// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements a function to report whether the DUT uses Vboot1 or Vboot2.

package reporters

import (
	"context"

	"chromiumos/tast/errors"
)

// Vboot2 determines whether the DUT's current firmware was selected by vboot2.
func (r *Reporter) Vboot2(ctx context.Context) (bool, error) {
	csValue, err := r.CrossystemParam(ctx, CrossystemParamFWVboot2)
	if err != nil {
		return false, err
	}
	return parseFWVboot2(csValue)
}

// parseFWVboot2 determines whether a fw_vboot2 crossystem value represents a vboot2 DUT.
func parseFWVboot2(value string) (bool, error) {
	if value != "1" && value != "0" {
		return false, errors.Errorf("unexpected fw_vboot2 value: want 1 or 0; got %q", value)
	}
	return value == "1", nil
}

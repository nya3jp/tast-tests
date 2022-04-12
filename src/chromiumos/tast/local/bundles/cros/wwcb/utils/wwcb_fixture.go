// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// InitFixtures reset all fixtures at the beginning of testing
// in case, between tests' fixture status affect each other, cause testing failed
func InitFixtures(ctx context.Context) error {
	testing.ContextLog(ctx, "Initialize fixtures")
	// disconnect all fixtures
	api := fmt.Sprintf("api/closeall")
	_, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return errors.Wrap(err, "failed to disconnect all fixtures")
	}
	// turn off docking station's power
	if err := ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to turn off docking station's power")
	}
	// turn on docking station's power
	if err := ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to turn on docking station's power")
	}
	return nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// WaitUntilCPUIdle waits until there is low CPU usage and the CPU temperature
// has dropped to a low level.
func WaitUntilCPUIdle(ctx context.Context, coolDownMode power.CoolDownMode) (CleanupCallback, error) {
	testing.ContextLog(ctx, "Waiting until CPU is idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return nil, err
	}

	err := power.WaitUntilCPUCoolDown(ctx, coolDownMode)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

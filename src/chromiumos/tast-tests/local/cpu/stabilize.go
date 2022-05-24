// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cpu

import "context"

// WaitUntilStabilized waits for the stabilization of the CPU.
// Currently, this waits for two conditions, one is the CPU's cooldown,
// and the other is the CPU idle.
func WaitUntilStabilized(ctx context.Context, cdConfig CoolDownConfig) error {
	if _, err := WaitUntilCoolDown(ctx, cdConfig); err != nil {
		return err
	}
	if err := WaitUntilIdle(ctx); err != nil {
		return err
	}
	return nil
}

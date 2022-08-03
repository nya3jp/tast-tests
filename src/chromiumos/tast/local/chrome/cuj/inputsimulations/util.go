// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputsimulations

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
)

func runActionFor(ctx context.Context, minDuration time.Duration, a action.Action) error {
	for endTime := time.Now().Add(minDuration); time.Now().Before(endTime); {
		if err := a(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

// RestartCras restart cras and waits for it to be ready
func RestartCras(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, "cras"); err != nil {
		return err
	}

	// Any device being available means CRAS is ready.
	if err := WaitForDevice(ctx, OutputStream|InputStream); err != nil {
		return errors.Wrap(err, "failed to wait for any output or input device")
	}

	return nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cups provides methods to coordinate with CUPS for printer handling.
package cups

import (
	"context"

	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/printing/printer"
	"go.chromium.org/chromiumos/tast-tests/local/upstart"
)

// RestartPrintingSystem restarts all of the printing-related processes, leaving the
// system in an idle state.
func RestartPrintingSystem(ctx context.Context) error {
	if err := printer.ResetCups(ctx); err != nil {
		return errors.Wrap(err, "failed to reset CUPS")
	}

	if err := upstart.RestartJob(ctx, "upstart-socket-bridge"); err != nil {
		return errors.Wrap(err, "failed to restart upstart-socket-bridge")
	}

	return nil
}

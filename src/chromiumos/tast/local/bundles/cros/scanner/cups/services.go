// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cups provides methods to coordinate with CUPS for printer handling.
package cups

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// EnsurePrinterIdle waits for CUPS to finish auto-configuring a newly added IPP-USB device,
// then shuts down the services that it may have automatically started.  This should leave
// the device connected and in an idle state.
func EnsurePrinterIdle(ctx context.Context, devInfo usbprinter.DevInfo) error {
	testing.ContextLog(ctx, "Waiting for printer to be configured")
	foundPrinterName, err := usbprinter.WaitPrinterConfigured(ctx, devInfo)
	if err != nil {
		return errors.Wrap(err, "failed to find printer name")
	}
	testing.ContextLog(ctx, "Printer configured with name: ", foundPrinterName)

	if err := ippusbbridge.Kill(ctx, devInfo); err != nil {
		return errors.Wrap(err, "failed to kill ippusb_bridge")
	}
	if err := upstart.StopJob(ctx, "ippusb"); err != nil {
		return errors.Wrap(err, "failed to stop ippusb service")
	}
	if err := printer.ResetCups(ctx); err != nil {
		return errors.Wrap(err, "failed to reset CUPS")
	}
	if err := upstart.RestartJob(ctx, "upstart-socket-bridge"); err != nil {
		return errors.Wrap(err, "failed to restart upstart-socket-bridge")
	}
	return nil
}

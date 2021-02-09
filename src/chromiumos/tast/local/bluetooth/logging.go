// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluetooth contains helpers to interact with the system's bluetooth
// adapters.
package bluetooth

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// SetDebugLogLevels sets the logging level for Bluetooth debug logs.
func SetDebugLogLevels(ctx context.Context, dispatcherVerbosity, newblueVerbosity, bluezVerbosity, kernelVerbosity byte) error {
	if err := testexec.CommandContext(ctx, "dbus-send", "--system", "--print-reply",
		"--dest=org.chromium.Bluetooth", "/org/chromium/Bluetooth", "org.chromium.Bluetooth.Debug.SetLevels",
		fmt.Sprintf("byte:%v", dispatcherVerbosity),
		fmt.Sprintf("byte:%v", newblueVerbosity),
		fmt.Sprintf("byte:%v", bluezVerbosity),
		fmt.Sprintf("byte:%v", kernelVerbosity),
	).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set bluetooth log levels")
	}
	return nil
}

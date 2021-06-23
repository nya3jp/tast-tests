// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluetooth contains helpers to interact with the system's bluetooth
// adapters.
package bluetooth

import (
	"context"
	"fmt"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// LogVerbosity indicates whether or not to enable verbose logging for the different bluetooth modules.
type LogVerbosity struct {
	Bluez  bool
	Kernel bool
}

// SetDebugLogLevels sets the logging level for Bluetooth debug logs.
func SetDebugLogLevels(ctx context.Context, levels LogVerbosity) error {
	btoi := map[bool]int{
		false: 0,
		true:  1,
	}
	if err := testexec.CommandContext(ctx, "dbus-send", "--system", "--print-reply",
		"--dest=org.bluez", "/org/chromium/Bluetooth", "org.chromium.Bluetooth.Debug.SetLevels",
		fmt.Sprintf("byte:%v", btoi[levels.Bluez]),
		fmt.Sprintf("byte:%v", btoi[levels.Kernel]),
	).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set bluetooth log levels")
	}
	return nil
}

// StartBTSnoopLogging starts capturing Bluetooth HCI "btsnoop" logs in a file at the specified path.
// Call Start on the returned command to start log collection, and call Kill when finished to end btmon.
func StartBTSnoopLogging(ctx context.Context, path string) *testexec.Cmd {
	return testexec.CommandContext(ctx, "/usr/bin/btmon", "-w", path)
}

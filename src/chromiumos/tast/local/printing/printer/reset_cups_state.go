// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package printer provides utilities about printer/cups.
package printer

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ResetCups removes the privileged directories for cupsd.
// If cupsd is running, this stops it.
// It also prevents cups from being reset during session changes.
func ResetCups(ctx context.Context) error {
	if err := ioutil.WriteFile("/run/cups/.disable", nil, 0644); err != nil {
		return err
	}
	// Wait for the stamp file to prevent race conditions, in case the script
	// is currently running.
	testing.ContextLog(ctx, "Waiting for stamp file")
	if err := testing.Poll(ctx, func(context.Context) error {
		_, err := os.Stat("/run/cups/.stamp")
		return err
	}, nil); err != nil {
		return err
	}
	return testexec.CommandContext(ctx, "/usr/share/cros/init/cups-clear-state.sh").Run(testexec.DumpLogOnError)
}

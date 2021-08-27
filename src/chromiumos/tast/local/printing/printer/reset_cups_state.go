// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package printer provides utilities about printer/cups.
package printer

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/upstart"
)

// ResetCups removes the privileged directories for cupsd.
// If cupsd is running, this stops it.
func ResetCups(ctx context.Context) error {
	if err := upstart.StopJob(ctx, "cupsd"); err != nil {
		return err
	}
	return testexec.CommandContext(ctx, "systemd-tmpfiles", "--remove", "/usr/lib/tmpfiles.d/chromeos.conf").Run(testexec.DumpLogOnError)
}

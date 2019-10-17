// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

const (
	wilcoSupportdJob = "wilco_dtc_supportd"
)

// StartSupportd starts the upstart process wilco_dtc_supportd.
func StartSupportd(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, wilcoSupportdJob); err != nil {
		return errors.Wrap(err, "unable to start the wilco_dtc_supportd service")
	}
	return nil
}

// StopSupportd stops the upstart process wilco_dtc_supportd.
func StopSupportd(ctx context.Context) error {
	if err := upstart.StopJob(ctx, wilcoSupportdJob); err != nil {
		return errors.Wrap(err, "unable to stop the wilco_dtc_supportd service")
	}
	return nil
}

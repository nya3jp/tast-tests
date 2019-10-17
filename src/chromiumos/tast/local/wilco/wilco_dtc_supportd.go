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
	wilcoSupportJob = "wilco_dtc_supportd"
)

// StartWilcoSupportDaemon starts the upstart process wilco_dtc_supportd.
func StartWilcoSupportDaemon(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, wilcoSupportJob); err != nil {
		return errors.Wrap(err, "wilco DTC Support daemon could not start")
	}
	return nil
}

// StopWilcoSupportDaemon stops the upstart process wilco_dtc_supportd.
func StopWilcoSupportDaemon(ctx context.Context) error {
	if err := upstart.StopJob(ctx, wilcoSupportJob); err != nil {
		return errors.Wrap(err, "unable to stop Wilco DTC Support daemon")
	}
	return nil
}

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

// SupportdPID gets the process id of wilco_dtc_supportd.
func SupportdPID(ctx context.Context) (pid int, err error) {
	_, _, pid, err = upstart.JobStatus(ctx, wilcoSupportdJob)

	return pid, err
}

// StartSupportd starts the upstart process wilco_dtc_supportd.
func StartSupportd(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, wilcoSupportdJob, upstart.WithArg("VMODULE_ARG", "*=3")); err != nil {
		return errors.Wrapf(err, "unable to start the %s service", wilcoSupportdJob)
	}
	return nil
}

// StopSupportd stops the upstart process wilco_dtc_supportd.
func StopSupportd(ctx context.Context) error {
	if err := upstart.StopJob(ctx, wilcoSupportdJob); err != nil {
		return errors.Wrapf(err, "unable to stop the %s service", wilcoSupportdJob)
	}
	return nil
}

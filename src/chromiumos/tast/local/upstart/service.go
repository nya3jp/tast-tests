// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package upstart

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// RestartJobAndWaitForService is a utility for restarting jobs that
// provide named dbus services. It restarts the |job| as normal and
// waits for the |serviceName| service to be available.
func RestartJobAndWaitForService(ctx context.Context, job, serviceName string) error {
	if err := RestartJob(ctx, job); err != nil {
		return errors.Wrapf(err, "failed to restart %s", job)
	}

	// Wait for service to be ready.
	if bus, err := dbusutil.SystemBus(); err != nil {
		return errors.Wrap(err, "failed to connect to the message bus")
	} else if err := dbusutil.WaitForService(ctx, bus, serviceName); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", serviceName)
	}
	return nil
}

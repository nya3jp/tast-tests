// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package runtimeprobe provides utilities for runtime_probe tests.
package runtimeprobe

import (
	"context"

	"github.com/godbus/dbus"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
)

// Probe uses D-Bus call to get result from runtime_probe with given request.
// Currently only users |chronos| and |debugd| are allowed to call this D-Bus function.
func Probe(ctx context.Context, request *rppb.ProbeRequest) (*rppb.ProbeResult, error) {
	const (
		// Define the D-Bus constants here.
		// Note that this is for the reference only to demonstrate how
		// to use dbusutil. For actual use, session_manager D-Bus call
		// should be performed via
		// chromiumos/tast/local/session_manager package.
		jobName       = "runtime_probe"
		dbusName      = "org.chromium.RuntimeProbe"
		dbusPath      = "/org/chromium/RuntimeProbe"
		dbusInterface = "org.chromium.RuntimeProbe"
		dbusMethod    = dbusInterface + ".ProbeCategories"
	)

	if err := upstart.EnsureJobRunning(ctx, jobName); err != nil {
		return nil, errors.Wrap(err, "runtime probe is not running")
	}
	defer upstart.StopJob(ctx, jobName)

	conn, obj, err := dbusutil.ConnectPrivateWithAuth(ctx, sysutil.ChronosUID, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	result := &rppb.ProbeResult{}
	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, request, result); err != nil {
		return nil, errors.Wrapf(err, "failed to call method %s", dbusMethod)
	}
	return result, nil
}

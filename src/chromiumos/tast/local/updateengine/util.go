// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides ways to interact with update_engine daemon and utilities.
package updateengine

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Update Engine related constants.
const (
	ClientBin   = "update_engine_client"
	JobName     = "update-engine"
	ServiceName = "org.chromium.UpdateEngine"
)

// StartDaemon will start the daemon, ignoring already running.
func StartDaemon(ctx context.Context) error {
	testing.ContextLog(ctx, "start daemon: ", JobName)
	return upstart.EnsureJobRunning(ctx, JobName)
}

// StopDaemon will stop the daemon.
func StopDaemon(ctx context.Context) error {
	testing.ContextLog(ctx, "stop daemon: ", JobName)
	return upstart.StopJob(ctx, JobName)
}

// WaitForService waits for the update-engine DBus service to be available.
func WaitForService(ctx context.Context) error {
	testing.ContextLog(ctx, "wait for service: ", JobName, " to be available")
	if bus, err := dbusutil.SystemBus(); err != nil {
		return errors.Wrap(err, "failed to connect to the message bus")
	} else if err := dbusutil.WaitForService(ctx, bus, ServiceName); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", ServiceName)
	}
	return nil
}

// StatusResult holds the update_engine status.
// TODO(kimjae): Update to use protos or json.
type StatusResult struct {
	LastCheckedTime int64
}

var reLastCheckedTime = regexp.MustCompile(`LAST_CHECKED_TIME=(.*)`)

// Status calls the DBus method to fetch update_engine's status.
func Status(ctx context.Context) (*StatusResult, error) {
	testing.ContextLog(ctx, "status: getting status from ", JobName)

	buf, err := testexec.CommandContext(ctx, ClientBin, "--status").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}

	match := reLastCheckedTime.FindStringSubmatch(string(buf))
	if match == nil {
		return nil, errors.New("status: failed to find last checked time")
	}

	i, err := strconv.ParseInt(string(match[1]), 10, 64)
	if err != nil {
		return nil, errors.New("status: failed to parse last checked time")
	}

	var status StatusResult
	status.LastCheckedTime = i

	return &StatusResult{
		LastCheckedTime: i,
	}, nil
}

// IsFeatureEnabled calls the DBus method to see if a feature in update_engine is enabled.
func IsFeatureEnabled(ctx context.Context, feature Feature) (bool, error) {
	testing.ContextLog(ctx, "is feature enabled: ", feature)

	buf, err := testexec.CommandContext(ctx, ClientBin, "--is_feature_enabled="+string(feature)).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to get feature")
	}

	b, err := strconv.ParseBool(string(buf))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool")
	}

	return b, nil
}

// ToggleFeature calls the DBus method to toggle a feature in update_engine.
func ToggleFeature(ctx context.Context, feature Feature, enable bool) error {
	testing.ContextLog(ctx, "toggle feature: ", feature, " to ", enable)

	var arg string
	if enable {
		arg = "--enable_feature=" + string(feature)
	} else {
		arg = "--disable_feature=" + string(feature)
	}

	if err := testexec.CommandContext(ctx, ClientBin, arg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to toggle feature")
	}

	return nil
}

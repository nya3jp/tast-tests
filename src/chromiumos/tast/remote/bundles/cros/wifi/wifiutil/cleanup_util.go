// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
)

const waitServiceIdleTime = 30 * time.Second

// WaitServiceIdle waits for the service in shill on DUT going to idle state
// for at most waitServiceIdleTime. This is useful for tests to ensure a clean
// state when starting or to leaving one part of verification.
// NOTE: If the test is to verify a specific disconnection, spawn watcher with
// tf.ExpectShillProperty before the trigger might be preferred.
func WaitServiceIdle(ctx context.Context, tf *wificell.TestFixture, servicePath string) error {
	ctx, cancel := context.WithTimeout(ctx, waitServiceIdleTime)
	defer cancel()
	props := []*wificell.ShillProperty{{
		Property:       shillconst.ServicePropertyState,
		ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_WAIT,
	}}
	wait, err := tf.WifiClient().ExpectShillProperty(ctx, servicePath, props, nil)
	if err != nil {
		return errors.Wrap(err, "failed to watch service state")
	}
	if _, err := wait(); err != nil {
		return errors.Wrap(err, "failed to wait for service idle")
	}
	return nil
}

// ReserveForWaitServiceIdle reserve time for WaitServiceIdle call.
func ReserveForWaitServiceIdle(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, waitServiceIdleTime)
}

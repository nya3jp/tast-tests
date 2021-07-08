// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

// TryConnect asks DUT to connect to an AP with the given ops.
// Then deconfig the AP and wait for disconnection without
// explicit disconnect call. The object path of connected
// service is returned.
// This simulates the behavior that a user connects to a WiFi
// network and then leaves its range.
func TryConnect(ctx context.Context, tf *wificell.TestFixture, ops []hostapd.Option) (retPath string, retErr error) {
	collectErr := func(err error) {
		if err == nil {
			return
		}
		if retErr == nil {
			retErr = err
		}
		retPath = ""
		testing.ContextLog(ctx, "TryConnect err: ", err)
	}

	var servicePath string
	defer func(ctx context.Context) {
		if servicePath == "" {
			// Not connected, just return.
		}
		if err := WaitServiceIdle(ctx, tf, servicePath); err != nil {
			collectErr(errors.Wrap(err, "failed to wait for DUT leaving the AP"))
		}
	}(ctx)
	ctx, cancel := ReserveForWaitServiceIdle(ctx)
	defer cancel()

	ap, err := tf.ConfigureAP(ctx, ops, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to configure the AP")
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			collectErr(errors.Wrap(err, "failed to deconfig the AP"))
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	resp, err := tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to the AP")
	}
	servicePath = resp.ServicePath

	if err := tf.VerifyConnection(ctx, ap); err != nil {
		return "", errors.Wrap(err, "failed to verify connection to the AP")
	}

	return servicePath, nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/testing"
)

// AssertWifiEnabled makes sure that Wifi is enabled on the DUT. This function sets up a
// WiFi AP and then asks DUT to connect. The DUT will ping the AP to confirm its functionality.
func AssertWifiEnabled(ctx context.Context, tf *wificell.TestFixture) (retErr error) {
	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to configure the ap")
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to deconfig the ap")
			} else {
				testing.ContextLog(ctx, "Failed to deconfig the ap: ", err)
			}
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	_, err = tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		return errors.Wrap(err, "failed to connect to WiFi")
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to disconnect WiFi")
			} else {
				testing.ContextLog(ctx, "Failed to deconfig the ap: ", err)
			}
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		return errors.Wrap(err, "failed to ping from the DUT")
	}

	return nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

var pollTimeout = 30 * time.Second
var pollInterval = time.Second

// BtStatus describes the desired Bluetooth state for the boot pref and adapter powered status.
type BtStatus bool

const (
	// BtOff refers to the bluetooth setting being off
	BtOff BtStatus = false
	// BtOn refers to the bluetooth setting being on
	BtOn = true
)

func statusString(status bool) string {
	if status {
		return "on"
	}
	return "off"
}

// PollBluetoothBootPref polls the DUT's saved bluetooth preference until the context deadline is exceeded or until a result is returned. If an unexpected result is seen, the function emits an error.
func PollBluetoothBootPref(ctx context.Context, btClient network.BluetoothServiceClient, expectedStatus BtStatus, credKey string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothBootPref(ctx, &network.GetBluetoothBootPrefRequest{Credentials: credKey}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if response.Persistent != bool(expectedStatus) {
			return testing.PollBreak(errors.Wrapf(err, "Bluetooth boot pref is %s, expected to be %s", statusString(response.Persistent), statusString(bool(expectedStatus))))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  pollTimeout,
		Interval: pollInterval,
	})
}

// PollBluetoothPoweredStatus polls the DUT's bluetooth adapter powered setting until the context deadline is exceeded or until the correct power setting is observed.
func PollBluetoothPoweredStatus(ctx context.Context, btClient network.BluetoothServiceClient, expectedStatus BtStatus) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPoweredFast(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if response.Powered != bool(expectedStatus) {
			return errors.Errorf("Bluetooth powered status is %s, expected to %s after boot", statusString(response.Powered), statusString(bool(expectedStatus)))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  pollTimeout,
		Interval: pollInterval,
	})
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// RequestScanAndWaitForReport requests scan and blocks until report is
// received.  Before requesting scan it first waits for the current scan, if any
// in progress, to finish just to make sure that the report received is for scan
// requested.
func RequestScanAndWaitForReport(ctx context.Context, s *testing.State) error {
	tf := s.FixtValue().(*wificell.TestFixture)

	if _, err := tf.WifiClient().WaitScanIdle(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to wait for scan to be idle")
	}
	wpaMonitor, stop, ctx, err := tf.StartWPAMonitor(ctx, wificell.DefaultDUT)
	if err != nil {
		return errors.Wrap(err, "failed to start WPA monitor")
	}
	defer stop()
	scanSuccess := false
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req := &wifi.RequestScansRequest{Count: 1}
	if _, err := tf.WifiClient().RequestScans(timeoutCtx, req); err != nil {
		return errors.Wrap(err, "failed to request scan")
	}
	for !scanSuccess {
		event, err := wpaMonitor.WaitForEvent(timeoutCtx)
		if err != nil {
			return errors.Wrap(err, "failed to wait for scan event")
		}
		if event == nil { // timeout
			return errors.New("Unable to get scan results")
		}
		_, scanSuccess = event.(*wpacli.ScanResultsEvent)
	}
	return nil
}

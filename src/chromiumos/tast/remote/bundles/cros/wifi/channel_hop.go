// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        ChannelHop,
		Desc:        "Verifies that the DUT, connected to a BSS on one channel will successfully re-connect when the AP changes channels",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func ChannelHop(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Error("Failed to collect logs: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	ssid := hostapd.RandomSSID("TAST_CHAN_HOP_")
	randAddr, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID")
	}
	origBSSID := randAddr.String()
	sharedOps := []hostapd.Option{
		hostapd.SSID(ssid),
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	// The AP configs we're going to use. The initAPOps one is which we
	// manual connect to, and the ones in reconntAPOps are expected to ge
	// auto-reconnected.
	initAPOps := append([]hostapd.Option{hostapd.Channel(1), hostapd.BSSID(origBSSID)}, sharedOps...)
	// Checking both channel jumping on the same BSSID and channel jumping
	// between BSSIDs, all inside the same SSID.
	reconnectAPOps := [][]hostapd.Option{
		append([]hostapd.Option{hostapd.Channel(6), hostapd.BSSID(origBSSID)}, sharedOps...),
		append([]hostapd.Option{hostapd.Channel(11), hostapd.BSSID(origBSSID)}, sharedOps...),
		// The next two use default unique BSSID (i.e. different from origBSSID and each other).
		append([]hostapd.Option{hostapd.Channel(3)}, sharedOps...),
		append([]hostapd.Option{hostapd.Channel(8)}, sharedOps...),
	}

	// Defining some internal utilities.

	const waitIdleTime = 30 * time.Second
	// waitIdle waits for service idle for at most waitIdleTime.
	waitIdle := func(ctx context.Context, servicePath string) error {
		ctx, cancel := context.WithTimeout(ctx, waitIdleTime)
		defer cancel()
		props := []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
			Method:         network.ExpectShillPropertyRequest_CHECK_WAIT,
		}}
		wait, err := tf.ExpectShillProperty(ctx, servicePath, props, nil)
		if err != nil {
			return errors.Wrap(err, "failed to watch service state")
		}
		if _, err := wait(); err != nil {
			return errors.Wrap(err, "failed to wait service idle")
		}
		return nil
	}

	// collectFirstErr is an utility function for collecting errors in defer.
	collectFirstErr := func(firstErr *error, err error) {
		if err == nil {
			return
		}
		if firstErr != nil {
			*firstErr = err
		}
		s.Log("Found error: ", err)
	}

	// Connect to the first AP. Wrap this in a function for easier cleanup.
	// The service path is returned so that we can wait for reconnection
	// later.
	var servicePath string
	err = func() (retErr error) {
		// As we don't DisconnectWifi, we'll need to wait for idle state after
		// DeconfigAP before leaving the function.
		defer func(ctx context.Context) {
			if servicePath == "" {
				// Not connected, just return.
			}
			if err := waitIdle(ctx, servicePath); err != nil {
				collectFirstErr(&retErr, errors.Wrap(err, "failed to wait for DUT leaving initial AP"))
			}
		}(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, waitIdleTime)
		defer cancel()

		ap, err := tf.ConfigureAP(ctx, initAPOps, nil)
		if err != nil {
			return errors.Wrap(err, "failed to configure the initial AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectFirstErr(&retErr, errors.Wrap(err, "failed to deconfig the initial AP"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		resp, err := tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			return errors.Wrap(err, "failed to connect to the initial AP")
		}
		servicePath = resp.ServicePath

		if err := tf.VerifyConnection(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to verify connection to the inital AP")
		}

		return nil
	}()
	if err != nil {
		s.Fatal("Failed to set up initial connection: ", err)
	}

	// Try start the APs in reconnectAPOps and verify DUT will reconnect to
	// the new AP.
	runOnce := func(ctx context.Context, apOps []hostapd.Option) (retErr error) {
		// Cancel the context inside to leave with a cleaner state.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		defer func(ctx context.Context) {
			s.Log("Waiting for service idle")
			if err := waitIdle(ctx, servicePath); err != nil {
				collectFirstErr(&retErr, errors.Wrap(err, "failed to wait for service idle"))
			}
		}(ctx)
		ctx, cancel = ctxutil.Shorten(ctx, waitIdleTime)
		defer cancel()

		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return errors.Wrap(err, "failed to configure the initial AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectFirstErr(&retErr, errors.Wrap(err, "failed to deconfig the initial AP"))
			}
		}(ctx)

		s.Log("Waiting for DUT to auto reconnect")
		// When power save is on, it might take long time for DUT to see
		// the beacons from the new AP. As we now have power save turned
		// on by default, let's trigger active scans which is a more
		// likely use case than turning power save off. (e.g. scans
		// triggered by UI)
		req := &network.WaitForReconnectRequest{
			ServicePath: servicePath,
			Timeout:     (30 * time.Second).Nanoseconds(),
			Scan:        true,
		}
		if _, err := tf.WifiClient().WaitForReconnect(ctx, req); err != nil {
			return errors.Wrap(err, "failed to wait for DUT to reconnect")
		}

		s.Log("Verifying connection")
		if err := tf.VerifyConnection(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to verify connection to the inital AP")
		}
		return nil
	}
	for i, ops := range reconnectAPOps {
		s.Logf("Trying AP setting #%d", i+1)
		if err := runOnce(ctx, ops); err != nil {
			s.Fatalf("Failed in testcase #%d: %v", i+1, err)
		}
	}
}

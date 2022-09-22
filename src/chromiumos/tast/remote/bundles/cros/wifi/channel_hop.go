// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/utils"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChannelHop,
		Desc: "Verifies that the DUT, connected to a BSS on one channel will successfully re-connect when the AP changes channels",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
		Timeout:     10 * time.Minute,
	})
}

func ChannelHop(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

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
	// manual connect to, and the ones in reconnectAPParams are expected
	// to be auto-reconnected.
	initAPOps := append([]hostapd.Option{hostapd.Channel(1), hostapd.BSSID(origBSSID)}, sharedOps...)
	// Checking both channel jumping on the same BSSID and channel jumping
	// between BSSIDs, all inside the same SSID.
	reconnectAPParams := []struct {
		desc string
		ops  []hostapd.Option
	}{
		{
			desc: "jump to ch6",
			ops:  append([]hostapd.Option{hostapd.Channel(6), hostapd.BSSID(origBSSID)}, sharedOps...),
		},
		{
			desc: "jump to ch11",
			ops:  append([]hostapd.Option{hostapd.Channel(11), hostapd.BSSID(origBSSID)}, sharedOps...),
		},
		// The next two use default unique BSSID (i.e. different from origBSSID and each other).
		{
			desc: "jump to ch3 with different BSSID",
			ops:  append([]hostapd.Option{hostapd.Channel(3)}, sharedOps...),
		},
		{
			desc: "jump to ch8 with different BSSID",
			ops:  append([]hostapd.Option{hostapd.Channel(8)}, sharedOps...),
		},
	}

	// Sets up AP with connection verification; then deconfigures the AP.
	var servicePath string
	err = func() (retErr error) {
		// Wait for the WiFi service to become idle, which is expected after
		// DeconfigAP() is called in the defer function below.
		defer func(ctx context.Context) {
			if servicePath == "" {
				// Not connected, just return.
			}
			if err := wifiutil.WaitServiceIdle(ctx, tf, servicePath); err != nil {
				utils.CollectFirstErr(ctx, &retErr, errors.Wrap(err, "failed to wait for DUT leaving initial AP"))
			}
		}(ctx)
		ctx, cancel := wifiutil.ReserveForWaitServiceIdle(ctx)
		defer cancel()

		ap, err := tf.ConfigureAP(ctx, initAPOps, nil)
		if err != nil {
			return errors.Wrap(err, "failed to configure the initial AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				utils.CollectFirstErr(ctx, &retErr, errors.Wrap(err, "failed to deconfig the initial AP"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		resp, err := tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			return errors.Wrap(err, "failed to connect to the initial AP")
		}
		servicePath = resp.ServicePath
		// No defer disconnect. This is what we're testing.

		if err := tf.VerifyConnection(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to verify connection to the inital AP")
		}

		return nil
	}()
	if err != nil {
		s.Fatal("Failed to set up initial connection: ", err)
	}

	// Try start the APs in reconnectAPParams and verify DUT will reconnect to
	// the new AP.
	runOnce := func(ctx context.Context, apOps []hostapd.Option) (retErr error) {
		// Cancel the context inside to leave with a cleaner state.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		defer func(ctx context.Context) {
			s.Log("Waiting for service idle")
			if err := wifiutil.WaitServiceIdle(ctx, tf, servicePath); err != nil {
				utils.CollectFirstErr(ctx, &retErr, errors.Wrap(err, "failed to wait for service idle"))
			}
		}(ctx)
		ctx, cancel = wifiutil.ReserveForWaitServiceIdle(ctx)
		defer cancel()

		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return errors.Wrap(err, "failed to configure the AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				utils.CollectFirstErr(ctx, &retErr, errors.Wrap(err, "failed to deconfig the AP"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		// We use CHECK_WAIT here instead of spawning watcher before ConfigureAP for
		// a more precise timeout. (Otherwise, timeout will include the time used
		// by ConfigureAP.)
		s.Log("Waiting for DUT to auto reconnect")
		// TODO(b/173339429): The timeout is quite long (90s) here because fast
		// scan after disconnection might not be available in some cases. It can
		// take DUT up to 60s (default scan interval) to see the new service.
		// In particular, the case is when DUT missed the DEAUTH frame sent on
		// AP deconfiguration, DUT will disconnect due to inactivity. In this
		// case, wpa_supplicant may send out BSSRemoved before CurrentBSS
		// change, which leads shill to go into DisconnectFrom logic instead of
		// HandleDisconnect and RestartFastScanAttempts will not be called.
		//
		// This maybe something to refine. We can shorten the timeout after it
		// is fixed or remove this TODO if it is WAI.
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		props := []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyIsConnected,
			ExpectedValues: []interface{}{true},
			Method:         wifi.ExpectShillPropertyRequest_CHECK_WAIT,
		}}
		wait, err := tf.WifiClient().ExpectShillProperty(ctx, servicePath, props, nil)
		if err != nil {
			return errors.Wrap(err, "failed to watch service state")
		}
		if _, err := wait(); err != nil {
			return errors.Wrap(err, "failed to wait for service connected")
		}

		s.Log("Verifying connection")
		if err := tf.VerifyConnection(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to verify connection to the AP")
		}
		return nil
	}
	for i, param := range reconnectAPParams {
		s.Logf("Trying #%d AP setting: %q", i+1, param.desc)
		if err := runOnce(ctx, param.ops); err != nil {
			s.Fatalf("Failed in testcase #%d: %v", i+1, err)
		}
	}
}

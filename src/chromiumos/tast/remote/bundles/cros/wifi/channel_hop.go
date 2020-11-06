// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

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
	// manual connect to, and the ones in reconntAPParams are expected
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

	// As we might have power save ON, it can take long time for
	// DUT to discover the new service. Shorten the scan interval
	// to 10 seconds to save some testing time.
	s.Log("Configuring scan setting")
	scanResp, err := tf.WifiClient().GetScanConfig(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get scan config: ", err)
	}
	oldConfig := scanResp.Config
	defer func(ctx context.Context) {
		s.Log("Restoring scan config: ", oldConfig)
		req := &network.SetScanConfigRequest{
			Config: oldConfig,
		}
		if _, err := tf.WifiClient().SetScanConfig(ctx, req); err != nil {
			s.Error("Failed to restore scan config to ", oldConfig)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	// Modify ScanInterval to 10s.
	req := &network.SetScanConfigRequest{Config: &network.ScanConfig{}}
	*req.Config = *oldConfig
	req.Config.ScanInterval = 10
	if _, err := tf.WifiClient().SetScanConfig(ctx, req); err != nil {
		s.Fatal("Failed to set scan config: ", err)
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

	// Sets up AP with connection verification; then deconfigures the AP.
	var servicePath string
	err = func() (retErr error) {
		// Wait for the WiFi service to become idle, which is expected after
		// DeconfigAP() is called in the defer function below.
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
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		// We use CHECK_WAIT here instead of spawning watcher before ConfigureAP for
		// a more precise timeout. (Otherwise, timeout will include the time used
		// by ConfigureAP.)
		s.Log("Waiting for DUT to auto reconnect")
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		props := []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyIsConnected,
			ExpectedValues: []interface{}{true},
			Method:         network.ExpectShillPropertyRequest_CHECK_WAIT,
		}}
		wait, err := tf.ExpectShillProperty(ctx, servicePath, props, nil)
		if err != nil {
			return errors.Wrap(err, "failed to watch service state")
		}
		if _, err := wait(); err != nil {
			return errors.Wrap(err, "failed to wait service idle")
		}

		s.Log("Verifying connection")
		if err := tf.VerifyConnection(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to verify connection to the inital AP")
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

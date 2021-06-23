// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type paramBgscanBackoff struct {
	ap1Ops []hostapd.Option
	ap2Ops []hostapd.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func: BgscanBackoff,
		Desc: "Verifies that bgscan aborts and/or backs off when there is consistent outgoing traffic",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
		Timeout:     6 * time.Minute, // This test has long ping time, assign a longer timeout.
		// Skip on Marvell on 8997 platforms because of test failure post security fixes b/187853331
		// Test failure is due to increased RTT time during Background scan backoff transition.
		HardwareDeps: hwdep.D(hwdep.WifiNotMarvell8997()),
		Params: []testing.Param{
			{
				Val: &paramBgscanBackoff{
					ap1Ops: []hostapd.Option{
						hostapd.Channel(1),
						hostapd.Mode(hostapd.Mode80211nMixed),
						hostapd.HTCaps(hostapd.HTCapHT20),
					},
					ap2Ops: []hostapd.Option{
						hostapd.Channel(36),
						hostapd.Mode(hostapd.Mode80211nMixed),
						hostapd.HTCaps(hostapd.HTCapHT20),
					},
				},
			},
			{
				// This test case verifies that bgscan aborts and/or backs off when
				// there is consistent outgoing traffic. This is a fork of the default test
				// that runs the test on channels 1 and 153 to serve two purposes:
				// (a) provide more 5 GHz coverage.
				// (b) help (a wee bit) to catch noise concerns around 5760 MHz as seen on
				// certain Intel SoCs.
				// This test can be compared with the default test, to see whether channel
				// 153 behaves worse than other 5GHz channels.
				Name: "5760noise_check",
				Val: &paramBgscanBackoff{
					ap1Ops: []hostapd.Option{
						hostapd.Channel(1),
						hostapd.Mode(hostapd.Mode80211nMixed),
						hostapd.HTCaps(hostapd.HTCapHT40),
					},
					ap2Ops: []hostapd.Option{
						hostapd.Channel(153),
						hostapd.Mode(hostapd.Mode80211nMixed),
						hostapd.HTCaps(hostapd.HTCapHT40),
					},
				},
			},
		},
	})
}

func BgscanBackoff(ctx context.Context, s *testing.State) {
	param := s.Param().(*paramBgscanBackoff)

	tf := s.FixtValue().(*wificell.TestFixture)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}
	// Turn off power save in this test as we are using ping RTT
	// as metric in this test. The default beacon interval (~100ms)
	// is too large compared with our threshold/margin and we'll
	// need much better resolution. Also, we don't want the timing
	// of beacons to bother our results.
	// e.g. default beacon interval is ~102ms and we might exceed
	// the 100ms threshold just because we send request right
	// after one beacon.
	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}
	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get the powersave mode: ", err)
	}
	if psMode {
		defer func(ctx context.Context) {
			s.Logf("Restoring power save mode to %t", psMode)
			if err := iwr.SetPowersaveMode(ctx, iface, psMode); err != nil {
				s.Errorf("Failed to restore powersave mode to %t: %v", psMode, err)
			}
		}(ctx)
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			s.Fatal("Failed to turn off powersave: ", err)
		}
	}

	// setupAndPing sets up an AP with param.ap1Option, connects DUT to it
	// with bgscan enabled/disabled and return ping statistics of the
	// connection. If bgscan is true, it further sets up another AP with
	// param.ap2Option and SSID of AP1 to make sure DUT can properly detect
	// it when there's no traffic.
	setupAndPing := func(ctx context.Context, bgscan bool) (ret *ping.Result, retErr error) {
		const (
			bgscanInterval    = 7    // In seconds.
			pingInterval      = 0.1  // In seconds.
			pingCountBgscan   = 1000 // Total 100 seconds of ping-ing.
			pingCountNoBgscan = 100  // Total 10 seconds of ping-ing.
			detectTimeout     = 50 * time.Second
		)

		// Utility function for collecting errors in defer.
		collectErr := func(err error) {
			if err == nil {
				return
			}
			s.Log("Error in setupAndPing: ", err)
			if retErr == nil {
				retErr = err
			}
		}

		ap1, err := tf.ConfigureAP(ctx, param.ap1Ops, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure AP1")
		}
		defer func(ctx context.Context) {
			s.Log("Deconfiguring AP1")
			if err := tf.DeconfigAP(ctx, ap1); err != nil {
				collectErr(errors.Wrap(err, "failed to deconfig AP1"))
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()

		s.Log("Configuring background scan")
		bgscanResp, err := tf.WifiClient().GetBgscanConfig(ctx, &empty.Empty{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get background scan config")
		}
		oldConfig := bgscanResp.Config
		var config wifi.BgscanConfig
		if bgscan {
			config = wifi.BgscanConfig{
				Method:        shillconst.DeviceBgscanMethodSimple,
				LongInterval:  bgscanInterval,
				ShortInterval: bgscanInterval,
			}
		} else {
			config = *bgscanResp.Config
			config.Method = shillconst.DeviceBgscanMethodNone
		}
		if _, err := tf.WifiClient().SetBgscanConfig(ctx, &wifi.SetBgscanConfigRequest{Config: &config}); err != nil {
			return nil, errors.Wrap(err, "failed to set background scan config")
		}
		defer func(ctx context.Context) {
			s.Log("Restoring bgscan config: ", oldConfig)
			req := &wifi.SetBgscanConfigRequest{
				Config: oldConfig,
			}
			if _, err := tf.WifiClient().SetBgscanConfig(ctx, req); err != nil {
				collectErr(errors.Wrap(err, "failed to set background scan config"))
			}
		}(ctx)
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Connecting to AP1")
		if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
			return nil, errors.Wrap(err, "failed to connect to WiFi")
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				collectErr(errors.Wrap(err, "failed to disconnect WiFi"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		pr := remoteping.NewRemoteRunner(s.DUT().Conn())
		var count int
		var desc string
		var pingLogPath string
		if bgscan {
			desc = "with bgscan"
			count = pingCountBgscan
			pingLogPath = "ping_bgscan.log"
		} else {
			desc = "without bgscan"
			count = pingCountNoBgscan
			pingLogPath = "ping_no_bgscan.log"
		}
		s.Logf("Pinging router %s, count=%d, interval=%f", desc, count, pingInterval)
		pingStats, err := pr.Ping(ctx, ap1.ServerIP().String(), ping.Count(count), ping.Interval(pingInterval), ping.SaveOutput(pingLogPath))
		if err != nil {
			return nil, errors.Wrap(err, "failed to ping router with bgscan")
		}
		s.Logf("Ping statistic %s: %v", desc, pingStats)

		if !bgscan {
			// No need to verify background scan, just return.
			return pingStats, nil
		}

		// Start AP2 with the same SSID as AP1.
		ssid := ap1.Config().SSID
		ap2Ops := append([]hostapd.Option{hostapd.SSID(ssid)}, param.ap2Ops...)
		ap2, err := tf.ConfigureAP(ctx, ap2Ops, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure AP1")
		}
		defer func(ctx context.Context) {
			s.Log("Deconfiguring AP2")
			if err := tf.DeconfigAP(ctx, ap2); err != nil {
				collectErr(errors.Wrap(err, "failed to deconfig AP2"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
		defer cancel()

		ap2MAC, err := legacyRouter.MAC(ctx, ap2.Interface())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get MAC address of %s on router", ap2.Interface())
		}

		s.Log("Waiting for AP2 to be found")
		req := &wifi.WaitForBSSIDRequest{
			Ssid:  []byte(ssid),
			Bssid: ap2MAC.String(),
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, detectTimeout)
		defer cancel()
		if _, err := tf.WifiClient().WaitForBSSID(timeoutCtx, req); err != nil {
			return nil, errors.Wrap(err, "failed to detect new BSS with background scan")
		}

		return pingStats, nil
	}

	// The lantency thresholds in ms to match the unit of ping.Result.
	const (
		latencyBaseline = 100
		// Dwell time for scanning is usually configured to be around 100 ms (some
		// are higher, around 150 ms), since this is also the standard beacon
		// interval. Tolerate spikes in latency up to 250 ms as a way of asking that
		// our PHY be servicing foreground traffic regularly during background scans.
		latencyMargin = 250
	)

	statsNoBgscan, err := setupAndPing(ctx, false)
	if err != nil {
		s.Fatal("Failed to measure latency without bgscan: ", err)
	}
	if statsNoBgscan.MaxLatency > latencyBaseline {
		s.Fatalf("RTT latency is too high even without background scans: %f ms > %f ms",
			statsNoBgscan.MaxLatency, float64(latencyBaseline))
	}

	statsBgscan, err := setupAndPing(ctx, true)
	if err != nil {
		s.Fatal("Failed to measure latency with bgscan: ", err)
	}
	diff := statsBgscan.MaxLatency - statsNoBgscan.MaxLatency
	if diff > latencyMargin {
		s.Fatalf("Significant increase in RTT due to bgscan: %f ms (bgscan) - %f ms (no bgscan) = %f ms > %f ms (RTT increase margin)",
			statsBgscan.MaxLatency, statsNoBgscan.MaxLatency, diff, float64(latencyMargin))
	}
}

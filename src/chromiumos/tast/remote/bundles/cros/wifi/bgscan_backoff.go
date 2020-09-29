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
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type paramBgscanBackoff struct {
	ap1Ops []hostapd.Option
	ap2Ops []hostapd.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        BgscanBackoff,
		Desc:        "Verifies that bgscan aborts and/or backs off when there is consistent outgoing traffic",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
		Timeout:     6 * time.Minute, // This test has long ping time, assign a longer timeout.
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

	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

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
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()

		s.Log("Configuring background scan")
		bgscanResp, err := tf.WifiClient().GetBgscanConfig(ctx, &empty.Empty{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get background scan config")
		}
		oldConfig := bgscanResp.Config
		var config network.BgscanConfig
		if bgscan {
			config = network.BgscanConfig{
				Method:        shillconst.DeviceBgscanMethodSimple,
				LongInterval:  bgscanInterval,
				ShortInterval: bgscanInterval,
			}
		} else {
			config = *bgscanResp.Config
			config.Method = shillconst.DeviceBgscanMethodNone
		}
		if _, err := tf.WifiClient().SetBgscanConfig(ctx, &network.SetBgscanConfigRequest{Config: &config}); err != nil {
			return nil, errors.Wrap(err, "failed to set background scan config")
		}
		defer func(ctx context.Context) {
			s.Log("Restoring bgscan config: ", oldConfig)
			req := &network.SetBgscanConfigRequest{
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
		if bgscan {
			desc = "with bgscan"
			count = pingCountBgscan
		} else {
			desc = "without bgscan"
			count = pingCountNoBgscan
		}
		s.Logf("Pinging router %s, count=%d, interval=%f", desc, count, pingInterval)
		pingStats, err := pr.Ping(ctx, ap1.ServerIP().String(), ping.Count(count), ping.Interval(pingInterval))
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

		ap2MAC, err := tf.Router().MAC(ctx, ap2.Interface())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get MAC address of %s on router", ap2.Interface())
		}

		s.Log("Waiting for AP2 to be found")
		req := &network.WaitForBSSIDRequest{
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

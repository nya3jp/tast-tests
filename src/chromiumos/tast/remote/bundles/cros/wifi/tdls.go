// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

type tdlsTestcase struct {
	testFunc func(ctx context.Context, peer1Conn, peer2Conn *ssh.Conn) error
	reverse  bool
}

const (
	ifName = "wlan0" // Needed for DUT/Peer symmetric calls.
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TDLS,
		Desc: "Tests of support for basic TDLS operation in the driver",
		Contacts: []string{
			"jck@semihalf.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Vars:        []string{"peer"},
		Params: []testing.Param{
			{
				// Basic test
				Val: []tdlsTestcase{{
					testFunc: tdlsDiscoverMAC,
				}, {
					testFunc: tdlsDiscoverMAC,
					reverse:  true,
				},
				},
			},
		}})
}

// tdlsDiscoverMAC tests support for TDLS Discover message for MAC Address.
func tdlsDiscoverMAC(ctx context.Context, peer1Conn, peer2Conn *ssh.Conn) error {
	peer2Addr, err := wifiutil.GetMAC(ctx, peer2Conn, ifName)
	if err != nil {
		return errors.Wrap(err, "failed to get peer MAC address, err")
	}
	testing.ContextLogf(ctx, "Peer MAC: %s", peer2Addr)
	_, err = wifiutil.WpaCli(ctx, peer1Conn, "OK", "tdls_discover", peer2Addr)
	if err != nil {
		return errors.Wrap(err, "failed TDLS Discover, err")
	}
	return nil
}

// TDLS main test.
func TDLS(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	peer, ok := s.Var("peer")
	if !ok || peer == "" {
		s.Fatal("Peer device address not declared properly")
	}

	apIface, err := tf.ConfigureAP(ctx,
		[]ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
		wpa.NewConfigFactory(
			"chromeos", wpa.Mode(wpa.ModePureWPA),
			wpa.Ciphers(wpa.CipherCCMP)))
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, apIface)
	defer cancel()
	s.Log("AP setup done")

	// Connect DUT.
	_, err = tf.ConnectWifiAP(ctx, apIface)
	if err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected DUT")

	// Connect the peer device.
	tp, err := wifiutil.NewTDLSPeer(ctx, peer, s.DUT().KeyFile(), s.DUT().KeyDir(), s.RPCHint())
	defer func(ctx context.Context) {
		if err := tp.Close(ctx); err != nil {
			s.Error("Failed to disconnect Peer device, err: ", err)
		}
	}(ctx)

	if _, err = tp.ConnectWifi(ctx, apIface); err != nil {
		s.Fatal("Failed to connect peer to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tp.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	// We don't use a separate reserve function, this is the same functionality as in tf.
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	peerConn := tp.Conn()
	s.Log("Connected Peer")

	// Setup capture.
	r, ok := apIface.Router().(support.Capture)
	if !ok {
		s.Fatalf("Router type %q does not have sufficient support for this test: ", apIface.Router().RouterTypeName())
	}

	apName := wifiutil.UniqueAPName()
	freqOps, err := apIface.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get Freq Opts, err: ", err)
	}
	capturer, err := r.StartCapture(ctx, apName, apIface.Config().Channel, freqOps)
	if err != nil {
		s.Fatal("Failed to start capturer, err: ", err)
	}
	defer func(ctx context.Context) {
		r.StopCapture(ctx, capturer)
	}(ctx)

	// Scan looking for SSID.
	runnerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ok, err = wifiutil.RunAndCheckOutput(runnerCtx, *peerConn.CommandContext(runnerCtx, "iwlist", ifName, "scan"), apIface.Config().SSID)
	if err != nil {
		s.Fatal("Failed to call iwlist scan, err: ", err)
	}

	// WPA_CLI - repeat scan. This just confirms WPA_CLI works.
	runnerCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = wifiutil.WpaCli(runnerCtx, peerConn, "OK", "scan")
	if err != nil {
		s.Fatal("Failed start scan through CLI, err: ", err)
	}

	runnerCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := wifiutil.WpaCli(ctx, peerConn, apIface.Config().SSID, "scan_results"); err != nil {
		s.Fatal("Failed to call wpa_cli, err: ", err)
	}

	// Run the actual TDLS testcase.
	testcases := s.Param().([]tdlsTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 130*time.Second)
			defer cancel()
			var err error
			// We want to test either active support (sending requests) as well as passive (handling requests).
			if tc.reverse {
				err = tc.testFunc(ctx, peerConn, s.DUT().Conn())
			} else {
				err = tc.testFunc(ctx, s.DUT().Conn(), peerConn)
			}
			if err != nil {
				s.Fatal("Failed to run test, err: ", err)
			}
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}

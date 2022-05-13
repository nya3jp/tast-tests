// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type tdlsPeer struct {
	dutIdx int
	conn   *ssh.Conn
	ifName string
	mac    net.HardwareAddr
	ip     net.IP
}

type tdlsTestcase struct {
	name     string
	testFunc func(ctx context.Context, tc *tdlsTestcase, peer1, peer2 tdlsPeer) error
	reverse  bool
	pcap     support.Capture
	tf       *wificell.TestFixture
	ap       *wificell.APIface
}

func init() {
	testing.AddTest(&testing.Test{
		Func: TDLS,
		Desc: "Tests of support for basic TDLS operation in the driver",
		Contacts: []string{
			"jck@semihalf.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell_dual_dut"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixtCompanionDut",
		HardwareDeps: hwdep.D(hwdep.WifiTDLS()),
		Params: []testing.Param{
			{
				// The test comprises of environment setup phase (~90sec) and testcase run (150ms-10s).
				// So the most optimal option for running multiple testcases would be to setup environment once then
				// run all test cases. For clarity test cases are organized in separate functions (with option
				// of running it in reverse direction). Also for clarity, each testcase has unique name
				// (for logging purpose only).
				Val: []tdlsTestcase{{
					name:     "discover_active",
					testFunc: tdlsDiscover,
				}, {
					name:     "discover_passive",
					testFunc: tdlsDiscover,
					reverse:  true,
				}, {
					name:     "setup_teardown_active",
					testFunc: tdlsSetupTeardown,
				}, {
					name:     "setup_teardown_passive",
					testFunc: tdlsSetupTeardown,
					reverse:  true,
				}},
			},
		}})
}

// tdlsDiscover tests support for TDLS Discover message.
func tdlsDiscover(ctx context.Context, tc *tdlsTestcase, peer1, peer2 tdlsPeer) error {
	// (Sub)Test steps:
	// 1. Run TDLS Discover (via wpa_cli), make sure the response is `OK`.
	//    (wpa_cli responds OK only after successful negotiation).
	testing.ContextLogf(ctx, "Peer MAC: %s", peer2.mac.String())

	// TODO(b/234845693): move to a dedicated remote runner.
	r := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: peer1.conn})
	err := r.TDLSDiscover(ctx, peer2.mac.String())
	if err != nil {
		return errors.Wrap(err, "failed TDLS Discover")
	}
	testing.ContextLog(ctx, "Success")
	return nil
}

// tdlsSetupTeardown tests support for TDLS Setup and Teardown messages.
func tdlsSetupTeardown(ctx context.Context, tc *tdlsTestcase, peer1, peer2 tdlsPeer) error {
	// (Sub)Test steps:
	// 1. Run control Ping to make sure the traffic passes.
	// 2. Start packet capture.
	// 3. Run TDLS Setup (via wpa_cli), make sure the response is `OK`.
	//    (wpa_cli responds OK only after successful negotiation).
	// 4. Run Ping.
	// 5. Run TDLS Teardown (via wpa_cli), make sure the response is `OK`.
	// 6. Make sure captured ICMP packets went through TDLS tunnel (TA/RA==SA/DA).
	testing.ContextLogf(ctx, "Peer 1 MAC: %s", peer1.mac.String())
	testing.ContextLogf(ctx, "Peer 2 MAC: %s, IP: %s", peer2.mac.String(), peer2.ip.String())
	result, err := tc.tf.PingFromSpecificDUT(ctx, peer1.dutIdx, peer2.ip.String(), ping.Interval(0.1))
	if err != nil {
		return err
	}
	if result.Received == 0 {
		return errors.New("no traffic passed through in initial test")
	}
	if result.Sent != result.Received {
		testing.ContextLogf(ctx, "WARNING: packets lost during transmission: %v<%v", result.Received, result.Sent)
	}

	freqOpts, err := tc.ap.Config().PcapFreqOptions()
	if err != nil {
		return errors.Wrap(err, "failed to get Freq Opts")
	}

	// Run ping and check Receiver address matches Destination (packets in tunnel).
	setupPingAndVerifyResults := func(ctx context.Context) (ret error) {
		// Setup TDLS tunnel
		// TODO(b/234845693): move to a dedicated remote runner.
		r := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: peer1.conn})
		err := r.TDLSSetup(ctx, peer2.mac.String())
		if err != nil {
			return errors.Wrap(err, "failed TDLS Setup")
		}
		testing.ContextLog(ctx, "TDLS Setup success")

		defer func(ctx context.Context) {
			// Teardown the tunnel.
			err = r.TDLSTeardown(ctx, peer2.mac.String())
			if ret == nil && err != nil {
				ret = errors.Wrap(err, "failed TDLS Teardown")
			}
			testing.ContextLog(ctx, "TDLS Teardown success")
		}(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, 1*time.Second)
		defer cancel()

		err = testing.Poll(ctx, func(ctx context.Context) error {
			err = r.TDLSLinkStatus(ctx, peer2.mac.String())
			if err != nil {
				return errors.Wrap(err, "wrong tdls link status")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
		if err != nil {
			return errors.Wrap(err, "failed to get status connected for the TDLS link")
		}

		result, err := tc.tf.PingFromSpecificDUT(ctx, peer1.dutIdx, peer2.ip.String(), ping.Interval(0.1))
		if err != nil {
			return err
		}
		if result.Sent != result.Received {
			testing.ContextLogf(ctx, "WARNING: packets lost during transmission: %v/%v", result.Sent-result.Received, result.Sent)
		}
		return wificell.VerifyPingResults(result, 0.8)
	}
	pcapPath, err := wifiutil.CollectPcapForAction(ctx, tc.pcap, tc.name, tc.ap.Config().Channel, freqOpts, setupPingAndVerifyResults)
	if err != nil {
		return errors.Wrap(err, "failed to collect pcap or perform action")
	}

	pkts, err := wifiutil.FindNonTDLSPackets(pcapPath, []net.HardwareAddr{peer1.mac, peer2.mac})
	if err != nil {
		return errors.Wrap(err, "failed to fiter packets")
	} else if len(pkts) > 0 {
		// Better to log offending packets once than to return them with error which gets printed multiple times.
		testing.ContextLogf(ctx, "Found invalid packets:%s", wifiutil.DumpPkts(pkts))
		return errors.New("invalid packets (not using TDLS tunnel) spotted")
	}
	testing.ContextLog(ctx, "Success")
	return nil
}

// TDLS is the main test routine.
func TDLS(ctx context.Context, s *testing.State) {
	// This test is expected to be an AVL check for TDLS support.
	// It requires a specific, dual-DUT setup:
	//                 +----+
	//         /------>| AP |<------\
	// +------+        +----+        +------+
	// | DUT1 |                      | DUT2 |
	// +------+                      +------+
	//
	// Test steps:
	// 1. Setup AP.
	// 2. For each of DUTs:
	// 2.1. Check TDLS support.
	// 2.2. Connect to AP.
	// 2.3. Read MAC and IP addresses.
	// 3. Start capture.
	// 4. Run subtests

	// We specifically want to configure AP in g mode, so that stations can negotiate a better standard.
	// This may spot some interesting failure modes.
	var tdlsAPOptions = []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.APSD()}

	tf := s.FixtValue().(*wificell.TestFixture)
	if tf.NumberOfDUTs() != 2 {
		s.Fatal("Test requires exactly two DUTs to be declared. Perhaps -companiondut=cd1:<host> is missing?")
	}

	// Reserve time for implicit call of tf.Close() in precondition after TDLS() returns.
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	apIface, err := tf.ConfigureAP(ctx, tdlsAPOptions, nil)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, apIface)
	defer cancel()
	s.Log("AP setup done")

	// Setup capture.
	pcap, ok := tf.Pcap().(support.Capture)
	if !ok {
		s.Fatalf("Router type %q does not have sufficient support for this test", apIface.Router().RouterType().String())
	}

	apName := tf.UniqueAPName()
	freqOps, err := apIface.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get Freq Opts: ", err)
	}
	capturer, err := pcap.StartCapture(ctx, apName, apIface.Config().Channel, freqOps)
	if err != nil {
		s.Fatal("Failed to start capturer: ", err)
	}
	defer func(ctx context.Context) {
		pcap.StopCapture(ctx, capturer)
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var tdlsPeers [2]tdlsPeer

	// Connect DUTs.
	for i := 0; i < 2; i++ {
		dutIdx := i // Work around the closure limitations.
		tdlsPeers[i].dutIdx = i

		// Check TDLS support. If one of the devices does not support it,
		// there's no point in continuing connection.
		err := wifiutil.CheckTDLSSupport(ctx, tf.DUTConn(dutIdx))
		if err != nil {
			s.Fatal("Failed to verify TDLS support: ", err)
		}

		_, err = tf.ConnectWifiAPFromDUT(ctx, dutIdx, apIface)
		if err != nil {
			s.Fatal("Failed to connect to WiFi: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectDUTFromWifi(ctx, dutIdx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		tdlsPeers[i].conn = tf.DUTConn(i)
		tdlsPeers[i].ifName, err = tf.DUTClientInterface(ctx, i)
		if err != nil {
			s.Fatal("Failed to get WiFi interface name: ", err)
		}
		tdlsPeers[i].mac, err = tf.DUTHardwareAddr(ctx, i)
		if err != nil {
			s.Fatal("Failed to get WiFi HW address: ", err)
		}
		ips, err := tf.DUTIPv4Addrs(ctx, i)
		if err != nil {
			s.Fatal("Failed to get WiFi IP address: ", err)
		}
		tdlsPeers[i].ip = ips[0]
		s.Logf("Connected DUT #%v", i+1)

		// This just confirms WPA_CLI works.
		// TODO(b/234845693): remove after stabilizing period.
		runnerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		r := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: tf.DUTConn(i)})
		_, err = r.Ping(runnerCtx, tdlsPeers[i].ifName)
		if err != nil {
			s.Fatal("Failed to ping wpa_cli: ", err)
		}

	}

	// Scan looking for SSID.
	// TODO(b/234845693): remove after stabilizing period.
	runnerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := wifiutil.Scan(runnerCtx, tf.DUTConn(1), tdlsPeers[1].ifName, apIface.Config().SSID); err != nil {
		s.Fatal("Failed to call iw scan: ", err)
	}

	// Run the actual TDLS testcase.
	testcases := s.Param().([]tdlsTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			tc.tf = tf
			tc.ap = apIface
			tc.pcap = pcap
			var err error
			// We want to test either active support (sending requests) as well as passive (handling requests).
			if !tc.reverse {
				err = tc.testFunc(testCtx, &tc, tdlsPeers[0], tdlsPeers[1])
			} else {
				err = tc.testFunc(testCtx, &tc, tdlsPeers[1], tdlsPeers[0])
			}
			if err != nil {
				s.Fatal("Failed to run subtest: ", err)
			}
		}
		s.Run(ctx, fmt.Sprintf("Testcase #%d: %s", i, tc.name), subtest)
	}
	s.Log("Tearing down")
}

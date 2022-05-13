// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/common/network/wpacli"
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

type tdlsTestcase struct {
	name     string
	testFunc func(ctx context.Context, tc *tdlsTestcase, peer1Conn, peer2Conn *ssh.Conn) error
	reverse  bool
	pcap     support.Capture
	tf       *wificell.TestFixture
	ap       *wificell.APIface
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
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixtCompanionDut",
		Vars:         []string{"peer"},
		HardwareDeps: hwdep.D(wifiutil.TDLSHwDeps()),
		Params: []testing.Param{
			{
				// Basic test
				Val: []tdlsTestcase{{
					name:     "Discover: active",
					testFunc: tdlsDiscover,
				}, {
					name:     "Discover: passive",
					testFunc: tdlsDiscover,
					reverse:  true,
				}, {
					name:     "Setup/Teardown: active",
					testFunc: tdlsSetupTeardown,
				}, {
					name:     "Setup/Teardown: passive",
					testFunc: tdlsSetupTeardown,
					reverse:  true,
				}},
			},
		}})
}

// tdlsDiscover tests support for TDLS Discover message.
func tdlsDiscover(ctx context.Context, tc *tdlsTestcase, peer1Conn, peer2Conn *ssh.Conn) error {
	peer2Addr, err := wifiutil.GetMAC(ctx, peer2Conn, ifName)
	if err != nil {
		return errors.Wrap(err, "failed to get peer MAC address, err")
	}
	testing.ContextLogf(ctx, "Peer MAC: %s", peer2Addr.String())
	r := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: peer1Conn})
	err = r.TDLSCmd(ctx, "tdls_discover", peer2Addr.String())
	if err != nil {
		return errors.Wrap(err, "failed TDLS Discover, err")
	}
	testing.ContextLog(ctx, "Success")
	return nil
}

// tdlsSetupTeardown tests support for TDLS Setup and Teardown messages.
func tdlsSetupTeardown(ctx context.Context, tc *tdlsTestcase, peer1Conn, peer2Conn *ssh.Conn) error {
	// Get the necessry addresses and data.
	peer1MAC, err := wifiutil.GetMAC(ctx, peer1Conn, ifName)
	if err != nil {
		return errors.Wrap(err, "failed to get peer 1 MAC address, err")
	}
	testing.ContextLogf(ctx, "Peer 1 MAC: %s", peer1MAC.String())
	peer2MAC, err := wifiutil.GetMAC(ctx, peer2Conn, ifName)
	if err != nil {
		return errors.Wrap(err, "failed to get peer 2 MAC address, err")
	}
	testing.ContextLogf(ctx, "Peer 2 MAC: %s", peer2MAC.String())
	peer2IP, err := wifiutil.GetIPv4(ctx, peer2Conn, ifName)
	if err != nil {
		return errors.Wrap(err, "failed to get peer IP address, err")
	}
	testing.ContextLogf(ctx, "Peer IP: %s", peer2IP)

	freqOpts, err := tc.ap.Config().PcapFreqOptions()
	if err != nil {
		return errors.Wrap(err, "failed to get Freq Opts, err")
	}

	result, err := wifiutil.PingFromHost(ctx, peer1Conn, peer2IP)
	if err != nil {
		return err
	}
	if result.Received == 0 {
		return errors.New("no traffic passed through in initial test")
	}
	if result.Sent != result.Received {
		testing.ContextLogf(ctx, "WARNING: packets lost during transmission: %v<%v", result.Received, result.Sent)
	}

	// Run ping and check Receiver address match Destination (packets in tunnel).
	pcapPath, err := wifiutil.CollectPcapForAction(ctx, tc.pcap, wifiutil.UniqueAPName(), tc.ap.Config().Channel, freqOpts,
		func(ctx context.Context) error {
			// Setup TDLS tunnel
			r := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: peer1Conn})
			err := r.TDLSCmd(ctx, "tdls_setup", peer2MAC.String())
			if err != nil {
				return errors.Wrap(err, "failed TDLS Setup, err")
			}
			testing.ContextLog(ctx, "Setup success")

			result, err := wifiutil.PingFromHost(ctx, peer1Conn, peer2IP)
			if err != nil {
				return err
			}
			if result.Received == 0 {
				return errors.New("no traffic passed through")
			}
			if result.Sent != result.Received {
				testing.ContextLogf(ctx, "WARNING: packets lost during transmission: %v<%v", result.Received, result.Sent)
			}
			// Teardown the tunnel.
			err = r.TDLSCmd(ctx, "tdls_teardown", peer2MAC.String())
			if err != nil {
				return errors.Wrap(err, "failed TDLS Teardown, err")
			}
			testing.ContextLog(ctx, "Teardown success")
			return nil
		})
	if err != nil {
		return errors.Wrap(err, "failed to collect pcap or perform action, err")
	}

	pkts, err := wifiutil.FindNonTDLSPackets(pcapPath, []net.HardwareAddr{peer1MAC, peer2MAC})
	if err != nil {
		return errors.Wrap(err, "failed to fiter packets, err")
	} else if len(pkts) > 0 {
		// Better to log offending packets once than to return them with error which gets printed multiple times.
		testing.ContextLogf(ctx, "Found invalid packets:%s", wifiutil.DumpPkts(pkts))
		return errors.New("invalid packets spotted")
	}
	testing.ContextLog(ctx, "Success")
	return nil
}

var tdlsAPOptions = []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)}

// TDLS main test.
func TDLS(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	if tf.DUTs() != 2 {
		s.Fatal("Test requires exactly two DUTs to be declared. Perhaps -companiondut=cd1:<host> is missing?")
	}

	apIface, err := tf.ConfigureAP(ctx, tdlsAPOptions, nil)
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

	// Connect DUTs.
	for i := 0; i < 2; i++ {
		dutIdx := i // Work around the closure limitations.

		// Check TDLS support. If one of the devices does not support it,
		// there's no point in continuing connection.
		wifiutil.CheckTDLSSupport(ctx, tf.DUTConn(dutIdx))
		if err != nil {
			s.Fatal("Failed to verify TDLS support, err: ", err)
		}

		_, err = tf.ConnectWifiAPFromDUT(ctx, dutIdx, apIface)
		if err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectDUTFromWifi(ctx, dutIdx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Logf("Connected DUT #%v", i+1)
	}

	// Setup capture.
	pcap, ok := tf.Pcap().(support.Capture)
	if !ok {
		s.Fatalf("Router type %q does not have sufficient support for this test: ", apIface.Router().RouterType().String())
	}

	apName := wifiutil.UniqueAPName()
	freqOps, err := apIface.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get Freq Opts, err: ", err)
	}
	capturer, err := pcap.StartCapture(ctx, apName, apIface.Config().Channel, freqOps)
	if err != nil {
		s.Fatal("Failed to start capturer, err: ", err)
	}
	defer func(ctx context.Context) {
		pcap.StopCapture(ctx, capturer)
	}(ctx)

	// Scan looking for SSID.
	runnerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ok, err = wifiutil.RunAndCheckOutput(runnerCtx, tf.DUTConn(1).CommandContext(runnerCtx, "iwlist", ifName, "scan"), apIface.Config().SSID)
	if err != nil {
		s.Fatal("Failed to call iwlist scan, err: ", err)
	}

	// WPA_CLI - repeat scan. This just confirms WPA_CLI works.
	runnerCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: tf.DUTConn(1)})
	_, err = r.Scan(runnerCtx)

	// Run the actual TDLS testcase.
	testcases := s.Param().([]tdlsTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testCtx, cancel := context.WithTimeout(ctx, 130*time.Second)
			defer cancel()
			tc.tf = tf
			tc.ap = apIface
			tc.pcap = pcap
			var err error
			// We want to test either active support (sending requests) as well as passive (handling requests).
			if tc.reverse {
				err = tc.testFunc(testCtx, &tc, tf.DUTConn(0), tf.DUTConn(1))
			} else {
				err = tc.testFunc(testCtx, &tc, tf.DUTConn(1), tf.DUTConn(0))
			}
			if err != nil {
				s.Fatal("Failed to run test, err: ", err)
			}
		}
		s.Run(ctx, fmt.Sprintf("Testcase #%d: %s", i, tc.name), subtest)
	}
	s.Log("Tearing down")
}

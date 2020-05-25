// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        BeaconInterval,
		Desc:        "Verifies that the beacon interval set on the AP is successfully adopted by the DUT",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router", "pcap"},
	})
}

func BeaconInterval(testCtx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	if router, _ := s.Var("router"); router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		tfOps = append(tfOps, wificell.TFPcap(pcap))
	}

	tf, err := wificell.NewTestFixture(testCtx, testCtx, s.DUT(), s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(dCtx context.Context) {
		if err := tf.Close(dCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}(testCtx)
	// Shorten testCtx on the way for each clean up in defer.
	testCtx, cancel := tf.ReserveForClose(testCtx)
	defer cancel()

	const expectBeaconInt = 200

	s.Log("Setting up AP")
	apOps := wificell.CommonAPOptions(hostapd.BeaconInterval(expectBeaconInt))
	ap, err := tf.ConfigureAP(testCtx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func(dCtx context.Context) {
		if err := tf.DeconfigAP(dCtx, ap); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}(testCtx)
	testCtx, _ = tf.ReserveForDeconfigAP(testCtx, ap)

	s.Log("Connecting to WiFi")
	if err := tf.ConnectWifi(testCtx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(dCtx context.Context) {
		if err := tf.DisconnectWifi(dCtx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(dCtx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ap.Config().Ssid, err)
		}
	}(testCtx)
	// Shorten a little bit for disconnect.
	testCtx, _ = ctxutil.Shorten(testCtx, 5*time.Second)

	s.Log("Start verification")
	// Check the beacon interval setting.
	iface, err := tf.ClientInterface(testCtx)
	if err != nil {
		s.Fatal("Failed to get DUT's WiFi interface: ", err)
	}
	bi, err := ifaceBeaconInt(testCtx, s.DUT(), iface)
	if err != nil {
		s.Fatal("Failed to get beacon interval: ", err)
	}
	if bi != expectBeaconInt {
		s.Fatalf("Unexpected beacon interval, got %d, want %d", bi, expectBeaconInt)
	}
	// Check connectivity.
	if err := tf.PingFromDUT(testCtx); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}

	s.Log("Verified; tearing down")
}

func ifaceBeaconInt(ctx context.Context, dut *dut.DUT, iface string) (int, error) {
	iwr := iw.NewRunner(dut.Conn())
	val, err := iwr.LinkValue(ctx, iface, "beacon int")
	if err != nil {
		return 0, err
	}
	bi, err := strconv.Atoi(val)
	if err != nil {
		return 0, errors.Wrapf(err, "beacon int %q not a number", val)
	}
	return bi, nil
}

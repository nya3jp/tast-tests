// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FastReconnectInDarkResume,
		Desc: "Verifies that the DUT can reconnect in one dark resume when got deauthenticated by AP",
		Contacts: []string{
			"yenlinlai@google.com",            // Test author.
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_suspend", "wificell_unstable"},
		VarDeps:     []string{"servo"},
		ServiceDeps: []string{wificell.TFServiceName},
		// TODO(b/187362093): Extend the platforms when WoWLAN is known to be good on them.
		HardwareDeps: hwdep.D(hwdep.Platform("volteer"), hwdep.ChromeEC()),
		Fixture:      "wificellFixt",
	})
}

func FastReconnectInDarkResume(ctx context.Context, s *testing.State) {
	// This test makes DUT connect to an AP and suspend. Then, ask AP to
	// deauthenticate the DUT and make sure the DUT can properly reconnect
	// in one dark resume and then go back to suspend.

	tf := s.FixtValue().(*wificell.TestFixture)

	// Set up the servo attached to the DUT.
	dut := s.DUT()
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}

	// Enable darkconnect.
	features := shillconst.WakeOnWiFiFeaturesDarkConnect
	ctx, restoreWakeOnWiFi, err := tf.WifiClient().SetWakeOnWifi(ctx, wificell.WakeOnWifiFeatures(features))
	if err != nil {
		s.Fatal("Failed to set up wake on WiFi: ", err)
	}
	defer func() {
		if err := restoreWakeOnWiFi(); err != nil {
			s.Error("Failed to restore wake on WiFi setting: ", err)
		}
	}()

	// We might respawn APs with the same options. Generate BSSIDs
	// by ourselves so that it won't be re-generated and will be
	// fixed in every usage.
	apOps := append(wificell.DefaultOpenNetworkAPOptions(),
		hostapd.SSID(hostapd.RandomSSID("FastReconnect_")))

	// Now set up AP and connect.
	s.Log("Set up AP")
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	s.Log("AP setup done")

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	// Spawn watcher for dark resume signal.
	watchDarkResumeRecv, err := tf.WifiClient().WatchDarkResume(ctx)
	if err != nil {
		s.Fatal("Failed to spawn dark resume watcher gRPC: ", err)
	}

	// Test the behaivor during suspend. Keep these in a function for easier cleanup/restore.
	func() {
		ctx, suspendCleanupFunc, err := wifiutil.DarkResumeSuspend(ctx, dut, pxy.Servo())
		if err != nil {
			s.Fatal("Failed to suspend the DUT: ", err)
		}
		defer func() {
			if err := suspendCleanupFunc(); err != nil {
				s.Error("Error in suspend cleanup: ", err)
			}
		}()
		s.Log("DUT suspended")

		// Now DUT is suspended. Deauthenticate DUT.
		// Note that this call is not synchronized and the STA might still
		// be regarded as connected before timeout, and we'll need to
		// identify the reconnection with connected time.
		deauthTime := time.Now()
		if err := ap.DeauthenticateClient(ctx, mac.String()); err != nil {
			s.Fatal("Failed to deauthenticate the DUT: ", err)
		}

		// Wait for the DUT to reconnect.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			staInfo, err := ap.STAInfo(ctx, mac.String())
			if err != nil {
				// The STA could disappear while reconnection,
				// don't PollBreak here.
				return err
			}
			if time.Now().Add(-staInfo.ConnectedTime).Before(deauthTime) {
				// This should be the old connection, keep on waiting.
				return errors.New("DUT not yet disconnect")
			}
			return nil
		}, &testing.PollOptions{
			Timeout:  20 * time.Second,
			Interval: 500 * time.Millisecond,
		}); err != nil {
			s.Fatal("Failed to wait for DUT to reconnect: ", err)
		}
		s.Log("DUT reconnected")

		if err := wifiutil.WaitDUTActive(ctx, pxy.Servo(), false, 20*time.Second); err != nil {
			s.Fatal("Failed to wait for DUT back to suspension after reconnect: ", err)
		}
		s.Log("DUT is back to suspension")
	}()

	resp, err := watchDarkResumeRecv()
	if err != nil {
		s.Fatal("Failed to get WatchDarkResuem response: ", err)
	}
	if resp.Count != 1 {
		s.Fatalf("Unexpected count of dark resume, got %d, want 1", resp.Count)
	}
}

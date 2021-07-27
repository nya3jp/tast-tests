// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type reconnectInDarkResumeParam struct {
	reconnectToSameAP       bool
	disconnectBeforeSuspend bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ReconnectInDarkResume,
		Desc: "Verifies that the DUT can reconnect to an autoconnectable AP during dark resume",
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
		Params: []testing.Param{
			{
				Name: "disconnect_after_suspend_diff_ap",
				Val: reconnectInDarkResumeParam{
					reconnectToSameAP:       false,
					disconnectBeforeSuspend: false,
				},
			}, {
				Name: "disconnect_after_suspend_same_ap",
				Val: reconnectInDarkResumeParam{
					reconnectToSameAP:       true,
					disconnectBeforeSuspend: false,
				},
			}, {
				Name: "disconnect_before_suspend_diff_ap",
				Val: reconnectInDarkResumeParam{
					reconnectToSameAP:       false,
					disconnectBeforeSuspend: true,
				},
			}, {
				Name: "disconnect_before_suspend_same_ap",
				Val: reconnectInDarkResumeParam{
					reconnectToSameAP:       true,
					disconnectBeforeSuspend: true,
				},
			},
		},
	})
}

func ReconnectInDarkResume(ctx context.Context, s *testing.State) {
	// This test sets up two autoconnectable services on the DUT and put the
	// DUT into suspend. Then, deconfigure and reconfigure the AP, and make
	// sure the DUT can properly reconnect to the autoconnect services in dark
	// resume. i.e. not fully wake up.

	tf := s.FixtValue().(*wificell.TestFixture)

	// Set up the servo attached to the DUT.
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

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
	var bssids []string
	for i := 0; i < 2; i++ {
		addr, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to generate BSSID: ", err)
		}
		bssids = append(bssids, addr.String())
	}
	apOps1 := append(wificell.DefaultOpenNetworkAPOptions(),
		hostapd.SSID(hostapd.RandomSSID("Reconnect_1_")),
		hostapd.BSSID(bssids[0]))
	apOps2 := append(wificell.DefaultOpenNetworkAPOptions(),
		hostapd.SSID(hostapd.RandomSSID("Reconnect_2_")),
		hostapd.BSSID(bssids[1]))

	// Try connecting to AP2 so that it is saved and could be reconnected
	// in later code.
	s.Log("Try connecting to AP2")
	if _, err := wifiutil.TryConnect(ctx, tf, apOps2); err != nil {
		s.Fatal("Failed to connect to AP2: ", err)
	}

	// Now set up AP1 and connect.
	s.Log("Set up AP1")
	ap, err := tf.ConfigureAP(ctx, apOps1, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	deconfigExistingAP := func(ctx context.Context) error {
		if ap == nil {
			return nil
		}
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			return err
		}
		ap = nil
		return nil
	}
	defer func(ctx context.Context) {
		if err := deconfigExistingAP(ctx); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
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
	s.Log("Connected")

	// Start testing.
	param := s.Param().(reconnectInDarkResumeParam)
	var reconnectOps []hostapd.Option
	var reconnectDesc string
	if param.reconnectToSameAP {
		reconnectOps = apOps1
		reconnectDesc = "the same"
	} else {
		reconnectOps = apOps2
		reconnectDesc = "the alternative"
	}

	if param.disconnectBeforeSuspend {
		if err := deconfigExistingAP(ctx); err != nil {
			s.Fatal("Failed to deconfig AP: ", err)
		}
		// No wait for service idle here. The intention is to allow
		// disconnect and suspend to race and then we can better make
		// sure that shill works properly regardless of the timing.
	}

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

	// Now DUT is suspended.
	// Deconfigure the existing AP if it still exists.
	if err := deconfigExistingAP(ctx); err != nil {
		s.Fatal("Failed to deconfig AP: ", err)
	}

	// Reconfigure an AP for DUT to reconnect.
	ap, err = tf.ConfigureAP(ctx, reconnectOps, nil)
	if err != nil {
		s.Fatalf("Failed to configure %s AP to reconnect: %v", reconnectDesc, err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		stas, err := ap.ListSTA(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, sta := range stas {
			if mac.String() == sta {
				return nil
			}
		}
		return errors.New("DUT not yet connected")
	}, &testing.PollOptions{
		Timeout:  time.Minute,
		Interval: 500 * time.Millisecond,
	}); err != nil {
		s.Fatal("Failed to wait for DUT to reconnect: ", err)
	}
	s.Log("DUT reconnected")

	if err := wifiutil.WaitDUTActive(ctx, pxy.Servo(), false, 20*time.Second); err != nil {
		s.Fatal("Failed to wait for DUT back to suspension after reconnect: ", err)
	}
	s.Log("DUT is back to suspension")
}

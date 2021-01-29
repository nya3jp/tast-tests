// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	remote_ping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type activeLinkParam struct {
	security security.ConfigFactory
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        LinkSuspend,
		Desc:        "Naive test for checking WiFi link behavior during suspend",
		Contacts:    []string{"yenlinlai@google.com"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars: []string{
			"router", "pcap", // For TestFixturePre.
			"wowlan",        // "false" if the test shouldn't setup WoWLAN, default true.
			"suspend_sec",   // time in seconds to suspend, default 30.
			"ping_interval", // ping interval in seconds, default 0.1.
		},
		Timeout: 10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "",
				Val: &activeLinkParam{
					security: nil, // Open network.
				},
			},
			{
				Name: "wpa",
				Val: &activeLinkParam{
					// Simple WPA setting.
					security: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModeMixed),
						wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						wpa.Ciphers2(wpa.CipherCCMP),
					),
				},
			},
			{
				Name: "wpa_gtk",
				Val: &activeLinkParam{
					security: wpa.NewConfigFactory(
						// Set shorter rekey period to test the rekey during suspend.
						"chromeos", wpa.Mode(wpa.ModeMixed),
						wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						wpa.Ciphers2(wpa.CipherCCMP),
						wpa.GTKRekeyPeriod(5),
						wpa.GMKRekeyPeriod(7),
					),
				},
			},
			{
				Name: "wpa_ptk",
				Val: &activeLinkParam{
					security: wpa.NewConfigFactory(
						// Set shorter rekey period to test the rekey during suspend.
						"chromeos", wpa.Mode(wpa.ModeMixed),
						wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						wpa.Ciphers2(wpa.CipherCCMP),
						wpa.PTKRekeyPeriod(5),
					),
				},
			},
		},
	})
}

func LinkSuspend(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	boolVar := func(key string, defaultVal bool) bool {
		str, ok := s.Var(key)
		if !ok {
			return defaultVal
		}
		val, err := strconv.ParseBool(str)
		if err != nil {
			s.Fatalf("Failed to parse %s=%s to boolean: %v", key, str, err)
		}
		return val
	}

	numVar := func(key string, defaultVal float64) float64 {
		str, ok := s.Var(key)
		if !ok {
			return defaultVal
		}
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			s.Fatalf("Failed to parse %s=%s to number: %v", key, str, err)
		}
		return val
	}

	// Testcase parameter.
	param := s.Param().(*activeLinkParam)
	// Parameters from Var.
	varParam := struct {
		wowlan       bool
		suspendSec   int
		pingInterval float64
	}{
		wowlan:       boolVar("wowlan", true),
		suspendSec:   int(numVar("suspend_sec", 30)),
		pingInterval: numVar("ping_interval", 0.1),
	}

	// Settings.
	apOps := []hostapd.Option{
		hostapd.SSID("TAST_TEST_LINK"),
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(36),
		hostapd.HTCaps(hostapd.HTCapHT40),
	}
	const pingStartBufSec = 2
	const pingTimeSec = 10

	// Set up AP.
	ap, err := tf.ConfigureAP(ctx, apOps, param.security)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func(ctx context.Context) {
		if ap == nil {
			// Already deconfigured.
			return
		}
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	s.Log("AP setup done")

	// Connect DUT to the network.
	if _, err = tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Log("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	// Connection sanity check.
	s.Log("Connected to the AP, checking connectivity")
	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed the connection: ", err)
	}

	if varParam.wowlan {
		s.Log("Set up WoWLAN")
		// Set up WoWLAN, currently via "iw wowlan enable".
		phy, err := tf.ClientPhy(ctx)
		if err != nil {
			s.Fatal("Failed to get client PHY name: ", err)
		}

		// Schedule restore first.
		defer func(ctx context.Context) {
			if err := s.DUT().Command("iw", phy, "wowlan", "disable").Run(ctx); err != nil {
				s.Error("Failed to restore wowlan to disable: ", err)
			}
		}(ctx)
		ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
		defer cancel()

		// TODO: Perhaps export some control to Var or Param.
		if err := s.DUT().Command("iw", phy, "wowlan", "enable", "disconnect", "4way-handshake").Run(ctx); err != nil {
			s.Fatal("Failed to set up WoWLAN: ", err)
		}

		// DEBUG: print out wowlan setting.
		if out, err := s.DUT().Command("iw", phy, "wowlan", "show").Output(ctx); err != nil {
			s.Fatal("Failed to get WoWLAN setting: ", err)
		} else {
			s.Log("Wake on WLAN setting: ", string(out))
		}
	}

	s.Log("Set up finished, start test suspend behavior")
	// Start the ping in background.
	var pingResult *ping.Result
	done := make(chan error, 1)
	// Wait the bgroutine on exit.
	defer func() { <-done }()
	// Close context to notify the routine to end.
	bgCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// TODO: need a better async version of ping to avoid the race of two threads.
	go func(ctx context.Context) {
		var err error
		defer close(done)
		done <- func() error {
			ops := []ping.Option{
				ping.Interval(varParam.pingInterval),
				ping.Count(int((pingStartBufSec + pingTimeSec) / varParam.pingInterval)),
				ping.SaveOutput("ping.log"),
			}
			pingResult, err = remote_ping.NewRemoteRunner(s.DUT().Conn()).Ping(ctx, ap.ServerIP().String(), ops...)
			return err
		}()
	}(bgCtx)
	// Short sleep to wait for ping to start.
	testing.Sleep(ctx, pingStartBufSec*time.Second)

	s.Logf("Going to suspend for %d seconds", varParam.suspendSec)
	start := time.Now()
	reason, err := s.DUT().Command(
		"powerd_dbus_suspend",
		"--print_wakeup_type",
		fmt.Sprintf("--suspend_for_sec=%d", varParam.suspendSec),
	).CombinedOutput(ctx)
	if err != nil {
		s.Fatal("Failed to suspend DUT: ", err)
	}
	timeDiff := time.Since(start)
	s.Logf("DUT is suspended for: %s, wake reason: %s", timeDiff.String(), strings.TrimSpace(string(reason)))

	// Get the ping result.
	if err := <-done; err != nil {
		s.Fatal("Failed to ping from DUT to AP: ", err)
	}
	s.Logf("%d out of %d packet received, loss: %f %%", pingResult.Received, pingResult.Sent, pingResult.Loss)
}

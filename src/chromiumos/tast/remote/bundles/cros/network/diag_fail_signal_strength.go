// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/conndiag"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

var roamNaturalSSID = hostapd.RandomSSID("TAST_ROAM_NAT_")

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagFailSignalStrength,
		Desc: "Tests that the WiFi signal strength network diagnostic test fails when the signal strength is below a threshold",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"cros-network-health@google.com", // network-health team
		},
		ServiceDeps:  []string{wificell.TFServiceName},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:wificell", "wificell_cq", "wificell_unstable"},
		Pre:          wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters | wificell.TFFeaturesAttenuator),
		Vars:         []string{"routers", "pcap", "attenuator"},
	})
}

// DiagFailSignalStrength tests that when the WiFi signal is attenuated, the WiFi
// signal strength network diagnostics routine fails.
func DiagFailSignalStrength(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	const channel = 1
	var apOpts = []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(channel),
		hostapd.HTCaps(hostapd.HTCapHT20),
		hostapd.BSSID("00:11:22:33:44:55"),
		hostapd.SSID(roamNaturalSSID),
	}

	_, freq, _ := wifiutil.ConfigureAP(ctx, s, apOpts, 0, nil)

	tf := s.PreValue().(*wificell.TestFixture)
	attenuator := tf.Attenuator()
	minAtten, err := attenuator.MinTotalAttenuation(channel)
	if err != nil {
		s.Fatal("Failed to get minimal attenuation")
	}
	if err := attenuator.SetTotalAttenuation(ctx, channel, minAtten, freq); err != nil {
		s.Fatal("Failed to set attenuation: ", err)
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	app, err := conndiag.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch connectivity diagnostics app: ", err)
	}

	conn, err := app.ChromeConn(ctx)
	if err != nil {
		s.Fatal("Failed to get network diagnostics mojo: ", err)
	}
	defer conn.Close()

	mojo, err := diag.NewMojoAPI(ctx, conn)
	if err != nil {
		s.Fatal("Unable to get network diagnostics mojo API: ", err)
	}
	defer mojo.Release(cleanupCtx)

	result, err := mojo.RunRoutine(ctx, diag.RoutineSignalStrength)
	if err != nil {
		s.Fatal("Failed to run routine: ", err)
	}

	const problemWeakSignal = 0
	if result.Verdict != diag.VerdictProblem {
		s.Fatalf("Expected routine problem verdict; got: %v, want: %v", result.Verdict, diag.VerdictProblem)
	}

	if len(result.Problems) != 1 {
		s.Fatalf("Unexpected problems length, got: %d, want: %d", result.Problems, 1)
	}

	if result.Problems[0] != problemWeakSignal {
		s.Fatalf("Routine reported unexpected problem; got %v, want %v", result.Problems[0], problemWeakSignal)
	}
}

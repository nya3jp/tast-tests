// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/conndiag"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

const (
	problemSecurityTypeNone     int = 0
	problemSecurityTypeWep8021x     = 1
	problemSecurityTypeWepPsk       = 2
	problemUnknownSecurityType      = 3
)

var secureWiFiSSID = hostapd.RandomSSID("TAST_SECURE_WIFI_")

type secureWiFiParams struct {
	SecConf  security.ConfigFactory
	Verdict  diag.RoutineVerdict
	Problems []int
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagSecureWifi,
		Desc: "Tests that the secure WiFi connection network diagnostic routine gives correct results with different WiFi security protocols",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"cros-network-health@google.com", // network-health team
		},
		ServiceDeps:  []string{wificell.TFServiceName},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:wificell", "wificell_cq", "wificell_unstable"},
		Pre:          wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters),
		Vars:         []string{"routers", "pcap"},
		Params: []testing.Param{{
			Name: "none",
			Val: secureWiFiParams{
				SecConf:  base.NewConfigFactory(),
				Verdict:  diag.VerdictProblem,
				Problems: []int{problemSecurityTypeNone},
			},
		}, {
			Name: "wep8021x",
			Val: secureWiFiParams{
				SecConf:  wep.NewConfigFactory([]string{"chromeos"}),
				Verdict:  diag.VerdictProblem,
				Problems: []int{problemSecurityTypeNone},
			},
		}, {
			Name: "wpa",
			Val: secureWiFiParams{
				SecConf:  wpa.NewConfigFactory("chromeos"),
				Verdict:  diag.VerdictNoProblem,
				Problems: []int{},
			},
		}},
	})
}

// DiagSecureWifi tests that the Secure WiFi connection network diagnostic
// routine returns the correct verdict when the WiFi AP uses a certain security
// protocol.
func DiagSecureWifi(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	params := s.Param().(secureWiFiParams)

	sc, err := params.SecConf.Gen()
	if err != nil {
		s.Fatal("Failed to generate security config: ", err)
	}

	const channel = 1
	var apOpts = []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(channel),
		hostapd.SecurityConfig(sc),
		hostapd.HTCaps(hostapd.HTCapHT20),
		hostapd.BSSID("00:11:22:33:44:55"),
		hostapd.SSID(secureWiFiSSID),
	}

	ap, _, _ := wifiutil.ConfigureAP(ctx, s, apOpts, 0, nil)
	tf := s.PreValue().(*wificell.TestFixture)

	disconnect := wifiutil.ConnectAP(ctx, s, ap, 0)
	defer disconnect(cleanupCtx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
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

	expectedResult := &diag.RoutineResult{
		Verdict:  params.Verdict,
		Problems: params.Problems,
	}
	if err := diag.CheckRoutineResult(result, expectedResult); err != nil {
		s.Fatal("Routine result did not match: ", err)
	}
}

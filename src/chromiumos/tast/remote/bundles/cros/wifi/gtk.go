// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GTK,
		Desc: "Verifies that we can continue to decrypt broadcast traffic while going through group temporal key (GTK) rekeys",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func GTK(ctx context.Context, s *testing.State) {
	// The settings gives us around 20 seconds to arping, which covers about 4 GTK rekeys.
	const (
		gtkRekeyPeriod = 5
		gmkRekeyPeriod = 7
		arpingCount    = 20
		totalTestTime  = 45 * time.Second
	)

	tf := s.FixtValue().(*wificell.TestFixture)

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211g),
		hostapd.Channel(1),
	}
	secConfFac := wpa.NewConfigFactory(
		"chromeos", wpa.Mode(wpa.ModeMixed),
		wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
		wpa.Ciphers2(wpa.CipherCCMP),
		wpa.UseStrictRekey(true),
		wpa.GTKRekeyPeriod(gtkRekeyPeriod),
		wpa.GMKRekeyPeriod(gmkRekeyPeriod),
	)
	ap, err := tf.ConfigureAP(ctx, apOps, secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("AP setup done")

	connectResp, err := tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	servicePath := connectResp.ServicePath
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	// Note that we wait for IsConnected to be false and check for a timeout
	// below since there's currently no better way to check that the Service
	// stayed connected through the duration of the test. The timeouts are
	// padded to ensure that we have enough time to process stream events
	// after the pings finish.
	props := []*wificell.ShillProperty{
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyIsConnected,
			Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
			ExpectedValues: []interface{}{true},
		},
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyIsConnected,
			Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
			ExpectedValues: []interface{}{false},
		},
	}
	pingBuffer := 5 * time.Second
	waitBuffer := 5 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, totalTestTime+pingBuffer+waitBuffer)
	defer cancel()
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props, []string{})

	pingCtx, cancel := ctxutil.Shorten(waitCtx, waitBuffer)
	defer cancel()
	if err := tf.PingFromDUT(pingCtx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	// Test that network traffic goes through.
	if err := tf.ArpingFromDUT(pingCtx, ap.ServerIP().String(), arping.Count(arpingCount)); err != nil {
		s.Error("Failed to send broadcast packets to server: ", err)
	}
	if err := tf.ArpingFromServer(pingCtx, ap.Interface(), arping.Count(arpingCount)); err != nil {
		s.Error("Failed to receive broadcast packets from server: ", err)
	}

	// We do a substring check here because the error loses it's type after
	// being marshaled through gRPC.
	if _, err := waitForProps(); err == nil || !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		s.Error("Failed to stay connected during rekeying process")
	}

	s.Log("Deconfiguring")
}

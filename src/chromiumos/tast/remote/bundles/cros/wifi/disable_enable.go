// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableEnable,
		Desc: "Tests that disabling and enabling WiFi re-connects the system",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_cq", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func DisableEnable(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	s.Log("AP setup done")

	connRes, err := tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get interface from the DUT: ", err)
	}

	// Start disabling and enabling WiFi interface.
	if _, err := tf.WifiClient().DisableEnableTest(ctx, &wifi.DisableEnableTestRequest{
		InterfaceName: iface,
		ServicePath:   connRes.ServicePath,
	}); err != nil {
		s.Fatal("DisableEnableTest failed: ", err)
	}

	// Verify the connection.
	// Note that we should have re-connected to the correct WiFi service after the DisableEnableTest call above.
	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}

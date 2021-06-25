// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WMM,
		Desc: "Verifies that the router and the DUT can handle multiple QoS levels",
		Contacts: []string{
			"billyzhao@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func WMM(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	s.Log("Setting up AP")
	ap, err := tf.DefaultOpenNetworkAP(ctx)
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

	s.Log("Connecting to WiFi")
	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	s.Log("Start verification")
	// Check connectivity for each QoS configuration.
	qosTypes := [4]ping.QOSType{ping.QOSBK, ping.QOSBE, ping.QOSVI, ping.QOSVO}
	for _, qos := range qosTypes {
		if err := tf.PingFromDUT(ctx, ap.ServerIP().String(), ping.QOS(qos)); err != nil {
			s.Fatalf("Failed to ping from DUT with QoS configuration %x: %v", qos, err)
		}

		if err := tf.PingFromServer(ctx, ping.QOS(qos)); err != nil {
			s.Fatalf("Failed to ping from the Server with QoS configuration %x: %v", qos, err)
		}
	}
	s.Log("Verified; tearing down")
}

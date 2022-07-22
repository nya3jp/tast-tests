// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        EnableRouterDNS,
		Desc:        "To do",
		Contacts:    []string{"tinghaolin@google.com", "chromeos-wifi-champs@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func EnableRouterDNS(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// DefaultOpenNetworkAPwithDNS configures the router with the default configuration and
	// starts the DHCP server.
	ap, err := tf.DefaultOpenNetworkAPwithDNS(ctx)
	if err != nil {
		s.Error("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	testing.ContextLogf(ctx, "WiFi SSID is: %s", ap.Config().SSID)

	if _, err := tf.ConnectWifi(ctx, ap.Config().SSID); err != nil {
		s.Error("Failed to connect to WiFi: ", err)
	}

	dutState, err := tf.WifiClient().QueryService(ctx)
	if err != nil {
		s.Fatal("Failed to query service: ", err)
	}
	testing.ContextLogf(ctx, "Debug: DUT WiFI name is: %s", dutState.Name)
	testing.ContextLogf(ctx, "Debug: DUT WiFI device is: %s", dutState.Device)
	testing.ContextLogf(ctx, "Debug: DUT WiFI type is: %s", dutState.Type)
	testing.ContextLogf(ctx, "Debug: DUT WiFI mode is: %s", dutState.Mode)
	testing.ContextLogf(ctx, "Debug: DUT WiFI state is: %s", dutState.State)

	testing.ContextLog(ctx, "Wait 10 seconds for presentation")
	testing.Sleep(ctx, 10)
}

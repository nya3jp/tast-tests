// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"net"

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
	router, err := tf.StandardRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	ap, err := tf.DefaultOpenNetworkAP(ctx)
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

	testing.ContextLog(ctx, "Debug: ap interface name is: ", ap.Interface())

	var (
		serverIP    = net.IPv4(192, 168, 0, 254)
		startIP     = net.IPv4(192, 168, 0, 1)
		endIP       = net.IPv4(192, 168, 0, 128)
		broadcastIP = net.IPv4(192, 168, 0, 255)
		mask        = net.IPv4Mask(255, 255, 255, 0)
	)
	router.EnableDNS(ctx, 53, []string{}, "#", net.IPv4(129, 0, 0, 1))
	ds, err := router.StartDHCP(ctx, "dhcp_dns", ap.Interface(), startIP, endIP, serverIP, broadcastIP, mask)
	if err != nil {
		s.Fatal("Failed to start the DHCP server: ", err)
	}

	defer func(ctx context.Context) {
		if err := router.StopDHCP(ctx, ds); err != nil {
			s.Error("Failed to stop the DHCP server: ", err)
		}
	}(ctx)
	ctx, cancel = ds.ReserveForClose(ctx)
	defer cancel()
}

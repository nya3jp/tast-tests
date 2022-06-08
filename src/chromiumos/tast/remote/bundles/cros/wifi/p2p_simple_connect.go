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
		Func: P2PSimpleConnect,
		Desc: "Tests P2P connection between two chromebooks",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		//Attr:        []string{"group:wificell_dual_dut"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtCompanionDut",
	})
}

func P2PSimpleConnect(ctx context.Context, s *testing.State) {
	/*
		This test checks the p2p connection between two chromebooks by using
		the following steps:
		1- Configures the main DUT as a p2p group owner (GO).
		2- Configures the Companion DUT as a p2p client.
		3- Connects the the p2p client to the GO network.
		4- Route the IP address in both GO and client.
		5- Verify the p2p connection.
		5-1- Run ping from the p2p GO.
		5-2- Run ping from the p2p client.
		6- Delete the IP route created in step 4.
		7- Deconfigure the p2p client.
		8- DEconfigure the p2p GO.
	*/
	tf := s.FixtValue().(*wificell.TestFixture)
	if err := tf.P2PConfigureGO(ctx); err != nil {
		s.Fatal("Failed to configure the p2p group owner (GO): ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.P2PDeconfigureGO(ctx); err != nil {
			s.Error("Failed to deconfigure the p2p group owner (GO): ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigP2P(ctx)
	defer cancel()

	if err := tf.P2PConfigureClient(ctx); err != nil {
		s.Fatal("Failed to configure the p2p client: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.P2PDeconfigureClient(ctx); err != nil {
			s.Error("Failed to deconfigure the p2p client: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigP2P(ctx)
	defer cancel()

	if err := tf.P2PConnect(ctx); err != nil {
		s.Fatal("Failed to connect the p2p client to the p2p group owner (GO) network: ", err)
	}

	if err := tf.P2PAddIPRoute(ctx); err != nil {
		s.Fatal("Failed to route the IP addresses in the p2p group owner (GO) and the p2p client: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.P2PDeleteIPRoute(ctx); err != nil {
			s.Error("Failed to delete the IP routing in the p2p group owner and p2p client: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeleteIPRoute(ctx)
	defer cancel()

	if err := tf.P2PAssertPingFromGO(ctx); err != nil {
		s.Fatal("Failed to ping the p2p client from the p2p group owner (GO): ", err)
	}

	if err := tf.P2PAssertPingFromClient(ctx); err != nil {
		s.Fatal("Failed to ping p2p group onwer (GO) from the p2p client: ", err)
	}
}

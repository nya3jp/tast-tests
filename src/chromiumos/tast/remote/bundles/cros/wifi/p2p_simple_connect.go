// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

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
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtCompanionDut",
	})
}

func P2PSimpleConnect(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	s.Log("P2P: Configure Group Owner.")
	if err := tf.P2PConfigureGO(ctx); err != nil {
		s.Fatal("Failed to set up the P2P Group Owner: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("P2P: Deconfigure Group Owner.")
		if err := tf.P2PDeconfigureGO(ctx); err != nil {
			s.Error("Failed to deconfig the P2P Group Owner: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigP2P(ctx)
	defer cancel()


	s.Log("P2P: Configure Client.")
	if err := tf.P2PConfigureClient(ctx); err != nil {
		s.Fatal("Failed to set up the P2P Client: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("P2P: Deconfigure Client.")
		if err := tf.P2PDeconfigureClient(ctx); err != nil {
			s.Error("Failed to deconfig the P2P Client: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigP2P(ctx)
	defer cancel()


	s.Log("P2P: Connect Client --> GO.")
	if err := tf.P2PConnect(ctx); err != nil {
		s.Fatal("Failed to connect to the group owner (GO) network: ", err)
	}


	s.Log("P2P: Route IP addresses.")
	if err := tf.P2PAddIPRoute(ctx); err != nil {
		s.Fatal("Failed to set up the group owner (GO): ", err)
	}
	defer func(ctx context.Context) {
		s.Log("P2P: Delete IP routes.")
		if err := tf.P2PDeleteIPRoute(ctx); err != nil {
			s.Error("Failed to deconfig the group owner (GO): ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeleteRoute(ctx)
	defer cancel()



	s.Log("P2P: Ping from GO to Client.")
	if err := tf.P2PAssertPingFromGO(ctx); err != nil {
		s.Fatal("Failed to ping Client from the GO: ", err)
	}


	s.Log("P2P: Ping from Client to GO.")
	if err := tf.P2PAssertPingFromClient(ctx); err != nil {
		s.Fatal("Failed to ping GO from the Client: ", err)
	}

	time.Sleep(10 * time.Second)

}

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

	if err := tf.P2PConfigureGO(ctx); err != nil {
		s.Fatal("Failed to set up the group owner (GO): ", err)
	}
	defer func() {
		if err := tf.P2PDeconfigerGO(); err != nil {
			s.Error("Failed to deconfig the group owner (GO): ", err)
		}
	}()
	ctx, cancel := tf.ReserveForDeconfigP2P(ctx)
	defer cancel()

	if err := tf.P2PConfigureClient(ctx); err != nil {
		s.Fatal("Failed to set up the group owner (GO): ", err)
	}
	defer func() {
		if err := tf.P2PDeconfigerClient(); err != nil {
			s.Error("Failed to deconfig the group owner (GO): ", err)
		}
	}()
	ctx, cancel = tf.ReserveForDeconfigP2P(ctx)
	defer cancel()

	if err := tf.P2PConnect(ctx); err != nil {
		s.Fatal("Failed to connect to the group owner (GO) network: ", err)
	}

	if err := tf.P2PAddIPRoute(ctx); err != nil {
		s.Fatal("Failed to set up the group owner (GO): ", err)
	}
	defer func() {
		if err := tf.P2PDeleteIPRoute(); err != nil {
			s.Error("Failed to deconfig the group owner (GO): ", err)
		}
	}()
	ctx, cancel = tf.ReserveForDeleteRoute(ctx, 2*time.Second)
	defer cancel()

	if err := tf.P2PAssertPingFromGO(ctx); err != nil {
		s.Fatal("Failed to ping Client from the GO: ", err)
	}

	if err := tf.P2PAssertPingFromClient(ctx); err != nil {
		s.Fatal("Failed to ping GO from the Client: ", err)
	}

}

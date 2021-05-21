// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AxPing,
		Desc: "Verifies DUT can associate and ping ax router on an open, hidden network.",
		Contacts: []string{
			"billyzhao@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		ServiceDeps: []string{wificell.TFServiceName},
		Vars:        []string{"router"},
	})
}

func AxPing(ctx context.Context, s *testing.State) {
	// Initialize TestFixture Options.
	var tfOps []wificell.TFOption
	if router, ok := s.Var("router"); ok && router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}
	tfOps = append(tfOps, wificell.TFRouterType(router.AxT))
	// Assert WiFi is up.
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to properly take down test fixture: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()
	var routerSettings = []router.AxRouterConfigParam{
		{
			Band:  router.Wl0,
			Key:   router.KeyAkm,
			Value: "",
		},
		{
			Band:  router.Wl0,
			Key:   router.KeyAuthMode,
			Value: "open",
		},
		{
			Band:  router.Wl0,
			Key:   router.KeySsid,
			Value: "googTest",
		},
		{
			Band:  router.Wl0,
			Key:   router.KeyChanspec,
			Value: "6l",
		},
		{
			Band:  router.Wl0,
			Key:   router.KeyClosed,
			Value: "1",
		},
		{
			Band:  router.Wl0,
			Key:   router.KeyRadio,
			Value: "1",
		},
	}
	if err = tf.SetAxSettings(ctx, routerSettings); err != nil {
		s.Error("Could not set ax settings ", err)
	}
	defer tf.DeconfigAxRouter(ctx, router.Wl0)

	ip, err := tf.GetAxRouterIPAddress(ctx)
	if err != nil {
		s.Error("Could not get ip addr ", err)
	}
	s.Logf("IP IS %s", ip)

	opts := append([]wificell.ConnOption{wificell.ConnHidden(true)})
	_, err = tf.ConnectWifi(ctx, "googTest", opts...)
	if err != nil {
		s.Error("Failed to connect to WiFi ", err)
	}

	if err := tf.PingFromDUT(ctx, ip); err != nil {
		s.Error("Failed to ping from the DUT ", err)
	}

}

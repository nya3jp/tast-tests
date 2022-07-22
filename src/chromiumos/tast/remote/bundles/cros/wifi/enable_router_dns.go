// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
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

	s.Log("Configure AP")
	ap, err := tf.DefaultOpenNetworkAPwithDNSHTTP(ctx)
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

	s.Log("Enable Portal Dection")
	if err := tf.WifiClient().SetPortalDetectionEnabled(ctx, true); err != nil {
		s.Error("Failed to enable portal dection: ", err)
	}

	s.Log("Start connecting WiFi")
	var servicePath string
	if connResp, err := tf.ConnectWifi(ctx, ap.Config().SSID); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	} else {
		servicePath = connResp.ServicePath
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	s.Log("Create a property watcher")
	props := []*wificell.ShillProperty{{
		Property:       shillconst.ServicePropertyState,
		ExpectedValues: []interface{}{shillconst.ServiceStateRedirectFound},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_WAIT,
	}}
	waitCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	waitForProps, err := tf.WifiClient().ExpectShillProperty(waitCtx, servicePath, props, nil)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher: ", err)
	}

	if _, err := waitForProps(); err != nil {
		s.Fatal("DUT: failed to wait for the properties: ", err)
	}

	s.Log("Disable Portal Dection")
	if err := tf.WifiClient().SetPortalDetectionEnabled(ctx, false); err != nil {
		s.Error("Failed to disable portal dection: ", err)
	}
}

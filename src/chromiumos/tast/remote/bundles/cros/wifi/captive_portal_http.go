// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strings"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

const (
	// httpScript is the file path to the script that executes the http server.
	httpScriptPath = "http/httpserver.py"
	cpTechnology   = "wifi"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CaptivePortalHTTP,
		Desc:        "Ensures that the service state transitions to the expected portal state based on the configured HTTP[S] probe response",
		Contacts:    []string{"tinghaolin@google.com", "chromeos-wifi-champs@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		Data:        []string{httpScriptPath},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

// CaptivePortalHTTP tests an end-to-end captive portal flow between the DUT and a remote router.
// The steps are listed below:
// 1. Configure the AP with DNS and HTTP capabilities
// 2. Check the initial state of CheckPortalList property and enable WiFi technology if not enabled
// 3. DUT connects to WiFi
// 4. Create a property watcher and check if the service property state is redirect-found
// 5. Restore the initial state of CheckPortalList
// 6. Release all resources
func CaptivePortalHTTP(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	ap, err := tf.DefaultOpenNetworkAPwithDNSHTTP(ctx, s.DataPath(httpScriptPath))
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
	s.Log("Configure AP")

	var cpList string
	var enablePortalDetection bool
	if cpList, err = tf.WifiClient().GetCaptivePortalList(ctx); err != nil {
		s.Fatal("Failed to get portal detection list: ", err)
	}

	enablePortalDetection = !strings.Contains(cpList, cpTechnology)
	if enablePortalDetection {
		if err := tf.WifiClient().SetPortalDetectionEnabled(ctx, true); err != nil {
			s.Error("Failed to enable portal detection: ", err)
		}
	}
	s.Log("Enable Portal Detection")

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
	s.Log("Connect to WiFi")

	props := []*wificell.ShillProperty{{
		Property:       shillconst.ServicePropertyState,
		ExpectedValues: []interface{}{shillconst.ServiceStateRedirectFound},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_WAIT,
	}}
	waitCtx, cancel := context.WithTimeout(ctx, shillconst.DefaultTimeout)
	defer cancel()
	waitForProps, err := tf.WifiClient().ExpectShillProperty(waitCtx, servicePath, props, nil)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher: ", err)
	}

	if _, err := waitForProps(); err != nil {
		s.Fatal("DUT: failed to wait for the properties: ", err)
	}

	if enablePortalDetection {
		if err := tf.WifiClient().SetCaptivePortalList(ctx, cpList); err != nil {
			s.Error("Failed to restore portal detectioni: ", err)
		}
	}
}

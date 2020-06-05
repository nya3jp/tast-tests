// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        MissingBeacons,
		Desc:        "Test how a DUT behaves when an AP disappears suddenly",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

const maxDisconnectTimeSeconds = 20

func MissingBeacons(fullCtx context.Context, s *testing.State) {
	/*
		Connects a DUT to an AP, then kills the AP in such a way that no de-auth
		message is sent.  Asserts that the DUT marks itself as disconnected from
		the AP within maxDisconnectTimeSeconds.
	*/
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(tfCtx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()
	ctx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap)
	defer cancel()

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func() {
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}()

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}
	/*
		Take down the AP interface, which looks like the AP "disappeared"
		from the DUT's point of view.  This is also much faster than actually
		tearing down the AP, which allows us to watch for the client reporting
		itself as disconnected.
	*/

	if err := tf.DisconnectWifi(ctx); err != nil {
		s.Fatal("DUT: failed to disconnect WiFi: ", err)
	}

}

// assertDisconnect asserts that we disconnect from ssid in maxDisconnectTimeSeconds.
func assertDisconnect(ctx context.Context, ssid string) error {
	// Leave some margin of seconds to check how long it actually
	// takes to disconnect should we fail to disconnect in time.
	timeout := maxDisconnectTimeSeconds + 10
	testing.ContextLogf(ctx, "Waiting %d seconds for client to notice the missing AP", timeout)

	// It seems redundant to disconnect a service that is already disconnected, but it prevents
	// shill from attempting to re-connect and foiling future connection attempts.
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}

	hexSSID := s.hexSSID(request.Ssid)

	props := map[string]interface{}{
		shill.ServicePropertyType:        shill.TypeWifi,
		shill.ServicePropertyWiFiHexSSID: hexSSID,
	}

	service, err = m.FindMatchingService(ctx, props)
	if err != nil {
		return err
	}

	// Spawn watcher before connect.
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	timeoutCtx, cancel := context.WithTimeout(ctx, maxDisconnectTimeSeconds*time.Second)
	defer cancel()
	if err := pw.Expect(timeoutCtx, shill.ServicePropertySSID, serviceStateIdle); err != nil {
		return err
	}
}

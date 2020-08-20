// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CSAReconnect,
		Desc:        "Verifies that DUT will switch to the new channel after the AP starts a CSA",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		// Run the test only on boards that support CSA.
		HardwareDeps: hwdep.D(hwdep.Platform("kukui", "jacuzzi", "grunt", "zork", "dedede", "speedy")),
	})
}

func CSAReconnect(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	const (
		primaryChannel = 64
		alterChannel   = 36
	)

	apOps := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(primaryChannel), hostapd.HTCaps(hostapd.HTCapHT20)}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	s.Log("AP setup done")
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	// Connect to the AP.
	var servicePath string
	if resp, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	} else {
		servicePath = resp.ServicePath
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("DUT: failed to disconnect WiFi: ", err)
		}
	}(ctx)
	s.Log("Connected")
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	// Assert connection.
	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	serverFreq, err := hostapd.ChannelToFrequency(alterChannel)
	if err != nil {
		s.Fatal("Failed to get server frequency")
	}

	// Start a shill properties watcher.
	props := []*wificell.ShillProperty{
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyWiFiFrequency,
			ExpectedValues: []interface{}{uint32(serverFreq)},
			Method:         network.ExpectShillPropertyRequest_CHECK_WAIT,
		},
	}
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	// Router send CSA.
	if err := ap.ChannelSwitch(ctx, alterChannel, 8, "ht"); err != nil {
		s.Fatal("Failed to send CSA from AP: ", err)
	}
	s.Log("CSA frame was sent from the AP")

	// Wait for the DUT to reconnect to the alternate channel.
	if err := waitForProps(); err != nil {
		s.Error("DUT: failed to wait for the properties, err: ", err)
	}

	// Assert connection.
	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MBOAssocDisallow,
		Desc: "Verifies that a DUT won't connect to an AP with the assoc disallow bit set",
		Contacts: []string{
			"matthewmwang@google.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Pre:          wificell.TestFixturePre(),
		Vars:         []string{"router", "pcap"},
		SoftwareDeps: []string{"mbo"},
	})
}

func MBOAssocDisallow(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	s.Log("Configuring AP")
	testSSID := hostapd.RandomSSID("MBO_ASSOC_DISALLOW_")
	apOpts := []hostapd.Option{hostapd.SSID(testSSID), hostapd.Mode(hostapd.Mode80211nMixed), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.Channel(1), hostapd.MBO()}
	ap, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("Setting assoc disallow")
	if err := ap.Set(ctx, hostapd.PropertyMBOAssocDisallow, "1"); err != nil {
		s.Fatal("Unable to set assoc disallow on AP: ", err)
	}

	s.Log("Attempting to connect to AP")
	if _, err = tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Log("Failed to connect as expected")
		return
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Fatal("Unexpectedly connected to AP")
}

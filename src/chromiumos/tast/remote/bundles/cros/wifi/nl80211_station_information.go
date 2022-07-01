// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Nl80211StationInformation,
		Desc: "Verify the support for nl80211 station information",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		HardwareDeps: hwdep.D(hwdep.WifiNotMarvell()),
		Fixture:      "wificellFixt",
	})
}

/*
nl80211 is the new 802.11 netlink interface public header. It provides station
information of an interface, such as "rx bytes", "tx bytes", "rx packets", "tx
packets".

This test uses the iw tool https://wireless.wiki.kernel.org/en/users/documentation/iw
to get the nl80211 station information of the WiFi interface, and checks if the
DUT supports all nl80211 fields required by the WiFi link statistics feature.
*/

func Nl80211StationInformation(ctx context.Context, s *testing.State) {
	// nl80211 station information fields required by the WiFi link statistics
	// feature
	nl80211Fields := []string{"rx bytes", "tx bytes", "rx packets", "tx packets", "rx drop misc", "tx retries", "tx failed", "signal"}
	tf := s.FixtValue().(*wificell.TestFixture)

	apOpts := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}
	apIface, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}

	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, apIface)
	defer cancel()
	s.Log("AP setup done")

	_, err = tf.ConnectWifiAP(ctx, apIface)
	if err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	// Get "ip dev interface station dump" output
	iwr := iw.NewRemoteRunner(s.DUT().Conn())
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}
	kvs, _ := iwr.AllStationInformation(ctx, clientIface)

	// Check if all required fields exist in the output
	var missingFields []string
	for _, field := range nl80211Fields {
		if _, ok := kvs[field]; !ok {
			missingFields = append(missingFields, field)
		}
	}
	if missingFields != nil {
		s.Fatalf("Missed %d nl80211_sta_info fields: %#v", len(missingFields), missingFields)
	}
}

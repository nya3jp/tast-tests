// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProfileBasic,
		Desc: "Tests basic operations on profiles and profile entries",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_cq", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func ProfileBasic(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	var aps []*wificell.APIface
	// It is restricted to configure multiple APs on the same phy, so start the APs on the different channels.
	for i, ch := range []int{1, 48} {
		ap, err := tf.ConfigureAP(ctx,
			[]hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ch), hostapd.HTCaps(hostapd.HTCapHT20)},
			wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
		)
		if err != nil {
			s.Fatalf("Failed to start AP%d: %v", i, err)
		}
		defer func(ctx context.Context, ap *wificell.APIface) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}(ctx, ap)
		var cancel context.CancelFunc
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		aps = append(aps, ap)
	}
	s.Log("APs setup done; start running test body")

	genShillProps := func(ap *wificell.APIface) protoutil.ShillValMap {
		props, err := ap.Config().SecurityConfig.ShillServiceProperties()
		if err != nil {
			s.Fatal("Failed to generate shill properties: ", err)
		}
		propsProto, err := protoutil.EncodeToShillValMap(props)
		if err != nil {
			s.Fatal("Failed to convert shill properties to protoutil.ShillValMap: ", err)
		}
		return propsProto
	}

	genRequestConfig := func(ap *wificell.APIface) *wifi.ProfileBasicTestRequest_Config {
		return &wifi.ProfileBasicTestRequest_Config{
			Ssid:          []byte(ap.Config().SSID),
			SecurityClass: ap.Config().SecurityConfig.Class(),
			Ip:            ap.ServerIP().String(),
			ShillProps:    genShillProps(ap),
		}
	}

	if _, err := tf.WifiClient().ProfileBasicTest(ctx, &wifi.ProfileBasicTestRequest{
		Ap0: genRequestConfig(aps[0]),
		Ap1: genRequestConfig(aps[1]),
	}); err != nil {
		s.Fatal("gRPC command ProfileBasicTest failed: ", err)
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        ProfileBasic,
		Desc:        "Tests basic operations on profiles and profile entries",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_cq", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

func ProfileBasic(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

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

	genRequestConfig := func(ap *wificell.APIface) *network.ProfileBasicTestRequest_Config {
		return &network.ProfileBasicTestRequest_Config{
			Ssid:       []byte(ap.Config().SSID),
			Security:   ap.Config().SecurityConfig.Class(),
			Ip:         ap.ServerIP().String(),
			ShillProps: genShillProps(ap),
		}
	}

	if _, err := tf.WifiClient().ProfileBasicTest(ctx, &network.ProfileBasicTestRequest{
		Ap0: genRequestConfig(aps[0]),
		Ap1: genRequestConfig(aps[1]),
	}); err != nil {
		s.Fatal("gRPC command ProfileBasicTest failed: ", err)
	}
}

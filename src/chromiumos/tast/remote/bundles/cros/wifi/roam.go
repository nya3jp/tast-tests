// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"strings"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type securityConfiguration struct {
	secConfFac security.ConfigFactory
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        Roam,
		Desc:        "Tests roaming to an AP that changes while the client is awake",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{ // Verifies that DUT can roam between two APs in full view of it.
				Name: "none",
				Val: securityConfiguration{
					secConfFac: nil,
				},
			}, {
				// Verifies that DUT can roam between two WPA APs in full view of it.
				Name: "wpa",
				Val: securityConfiguration{
					secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				},
			}, {
				// Verifies that DUT can roam between two WEP APs in full view of it.
				Name: "wep",
				Val: securityConfiguration{
					secConfFac: wep.NewConfigFactory([]string{"fedcba9876"}, wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
				},
			}, {
				// Verifies that DUT can roam between two 802.1x EAP-TLS APs in full view of it.
				Name: "eap",
				Val: securityConfiguration{
					secConfFac: nil,
				},
			},
		},
	})
}

func Roam(fullCtx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Error("Failed to tear down test fixture: ", err)
		}
	}()

	ap1Ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	// Configurer the initial AP.
	optionsAP1 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)}
	ap1, err := tf.ConfigureAP(ap1Ctx, optionsAP1, s.Param().(securityConfiguration).secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(ap1Ctx, ap1); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}()
	ap2Ctx, cancel := tf.ReserveForDeconfigAP(ap1Ctx, ap1)
	defer cancel()
	s.Log("AP1 setup done")

	// Connect to the initial AP.
	if _, err := tf.ConnectWifiAP(ap2Ctx, ap1); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(ap2Ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap1.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ap2Ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap1.Config().SSID, err)
		}
	}()
	s.Log("Connected to AP1")

	if err := tf.VerifyConnection(ap2Ctx, ap1); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	// Configurer the second AP.
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ap1.Config().SSID)}
	ap2, err := tf.ConfigureAP(ap2Ctx, optionsAP2, s.Param().(securityConfiguration).secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(ap2Ctx, ap2); err != nil {
			// Do not fail on this error as we're triggering some
			// deconfiguration in this test and the ap can be
			// deconfigured at this point.
			s.Log("Failed to deconfig ap (The ap might have been already deconfigured, as the test is triggering some deconfiguration): ", err)
		}
	}()
	ctx, cancel := tf.ReserveForDeconfigAP(ap2Ctx, ap1)
	defer cancel()
	s.Log("AP2 setup done")

	// Deconfigure the initial AP.
	if err := tf.DeconfigAP(ctx, ap1); err != nil {
		s.Error("Failed to deconfig ap, err: ", err)
	}
	s.Log("Deconfigured AP1")

	props := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: strings.ToUpper(hex.EncodeToString([]byte(ap2.Config().SSID))),
	}

	if err := tf.WaitForConnection(ctx, props); err != nil {
		s.Fatal("DUT: failed to connect to the second AP: ", err)
	}

	if err := tf.VerifyConnection(ctx, ap2); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}

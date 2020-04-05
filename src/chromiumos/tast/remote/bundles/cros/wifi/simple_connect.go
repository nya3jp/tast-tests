// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/security"
	"chromiumos/tast/remote/wificell/security/wep"
	"chromiumos/tast/remote/wificell/security/wpa"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type simpleConnectTestcase struct {
	apOptions       []hostapd.Option
	securityFactory security.Factory
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wifi_func"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name: "80211a",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(48)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(64)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(6)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channels 1, 6, 11 with a channel width of 20MHz.
				Name: "80211n24ht20",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT20)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(11), hostapd.HTCaps(hostapd.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channel 6 with a channel width of 40MHz.
				Name: "80211n24ht40",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channel 48 with a channel width of 20MHz.
				Name: "80211n5ht20",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channel 48
				// (40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT40Minus)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211ac5vht20",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(60),
						hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
					}},
				},
				// TODO(crbug.com/1024554): remove it after Monroe platform is end-of-life.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("Monroe")),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211ac5vht40",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(120), hostapd.HTCaps(hostapd.HTCapHT40),
						hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
					}},
				},
				// TODO(crbug.com/1024554): remove it after Monroe platform is end-of-life.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("Monroe")),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211ac5vht80mixed",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acMixed), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT40Plus),
						hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(42), hostapd.VHTChWidth(hostapd.VHTChWidth80),
					}},
				},
				// TODO(crbug.com/1024554): remove it after Monroe platform is end-of-life.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("Monroe")),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211ac5vht80pure",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(157), hostapd.HTCaps(hostapd.HTCapHT40Plus),
						hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(155), hostapd.VHTChWidth(hostapd.VHTChWidth80),
					}},
				},
				// TODO(crbug.com/1024554): remove it after Monroe platform is end-of-life.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("Monroe")),
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with TKIP.
				Name: "wpatkip",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with AES based CCMP.
				Name: "wpaccmp",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with both AES based CCMP and TKIP.
				Name: "wpamuti",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2.
				Name: "wpa2tkip",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure2),
							wpa.Ciphers2(wpa.CipherTKIP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for WPA2 (aka RSN) and encrypted under AES.
				Name: "wpa2",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2.
				Name: "wpamixed",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModeMixed),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected 802.11ac network supporting for WPA.
				Name: "wpa5vht80",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{
							hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT40Plus),
							hostapd.VHTCenterChannel(42), hostapd.VHTChWidth(hostapd.VHTChWidth80),
						},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						),
					},
				},
				// TODO(crbug.com/1024554): remove it after Monroe platform is end-of-life.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("Monroe")),
			}, {
				// Verifies that DUT can connect to an protected network whose WPA passphrase can be pure unicode, mixed unicode and ASCII, and all the punctuations.
				Name: "wpaoddpassphrase",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89", wpa.Mode(wpa.ModePure2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"abcdef\xc2\xa2", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							"abcdef\xc2\xa2", wpa.Mode(wpa.ModePure2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wpa.NewFactory(
							" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~", wpa.Mode(wpa.ModePure2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an WEP network with both open and shared system authentication and 40-bit pre-shared keys.
				Name: "wep40",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an WEP network with both open and shared system authentication and 104-bit pre-shared keys.
				Name: "wep104",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "mlk:ihgfedcba", "d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b", "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: []simpleConnectTestcase{
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6), hostapd.Hidden(true)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(36), hostapd.Hidden(true)}},
					{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.Hidden(true)}},
				},
			}, {
				// Verifies that DUT can connect to an hidden network supporting for WPA with TKIP, WPA with TKIP/AES, WPA2 with AES, and mixed WPA with TKIP/AES and WPA2 with AES.
				Name: "hiddenwpatkip",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModePure2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wpa.NewFactory(
							"chromeos", wpa.Mode(wpa.ModeMixed),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an hidden WEP network with open/shared system authentication and 40/104-bit pre-shared keys.
				Name: "hiddenwep",
				Val: []simpleConnectTestcase{
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsOpen),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
					{
						apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
						securityFactory: wep.NewFactory(
							[]string{"0123456789abcdef0123456789", "89abcdef0123456789abcdef01", "fedcba9876543210fedcba9876", "109fedcba987654321fedcba98"},
							wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgsShared),
						),
					},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	testOnce := func(ctx context.Context, s *testing.State, options []hostapd.Option, fac security.Factory) {
		ap, err := tf.ConfigureAP(ctx, options, fac)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
			}
		}()
		s.Log("Connected")

		ping := func(ctx context.Context) error {
			return tf.PingFromDUT(ctx)
		}

		if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
			s.Fatal("Failed to ping from DUT, err: ", err)
		}
		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
	}

	testcases := s.Param().([]simpleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.apOptions, tc.securityFactory)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}

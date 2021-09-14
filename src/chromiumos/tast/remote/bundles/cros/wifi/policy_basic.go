// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/json"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/tunneled1x"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/ptypes/empty"
)

const userGUID = "user_network_config"
const deviceGUID = "device_network_config"

var userApSSID = ap.RandomSSID("TAST_TEST_USER_")
var userApOpts = []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.SSID(userApSSID)}
var userSecConfFac = tunneled1x.NewConfigFactory(
	eapCert1.CACred.Cert, eapCert1.ServerCred, eapCert1.CACred.Cert, "tast-user@managedchrome.com", "test0000",
	tunneled1x.Mode(wpa.ModePureWPA2),
	tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
	tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
)
var userNetPolicy = policy.OpenNetworkConfiguration{
	Val: &policy.OpenNetworkConfigurationValue{
		NetworkConfigurations: []*policy.NetworkConfiguration{
			{
				GUID: userGUID,
				Name: "UserNetworkConfig",
				Type: "WiFi",
				WiFi: &policy.WiFi{
					AutoConnect: true,
					Security:    "WPA-EAP",
					SSID:        userApSSID,
					EAP: &policy.EAP{
						Outer:        "PEAP",
						Inner:        "MSCHAPv2",
						Identity:     "${LOGIN_EMAIL}",
						Password:     "${PASSWORD}",
						UseSystemCAs: false,
					},
				},
			},
		},
	},
}

type policyBasicTestcase struct {
	name          string
	sameAp        bool
	devApOpts     []ap.Option
	devSecConfFac security.ConfigFactory
	policy        *policy.DeviceOpenNetworkConfiguration
}

var deviceApSSID = ap.RandomSSID("TAST_TEST_DEVICE_")
var testCases = []policyBasicTestcase{
	{
		name:      "eap-user-switch",
		sameAp:    true,
		devApOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(userApSSID)},
		devSecConfFac: tunneled1x.NewConfigFactory(
			eapCert1.CACred.Cert, eapCert1.ServerCred, eapCert1.CACred.Cert, "testuser", "password",
			tunneled1x.Mode(wpa.ModePureWPA2),
			tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
			tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
			tunneled1x.Phase2User("tast-user@managedchrome.com", "test0000", tunneled1x.Layer2TypeMSCHAPV2),
		),
		policy: &policy.DeviceOpenNetworkConfiguration{
			Val: &policy.DeviceOpenNetworkConfigurationValue{
				NetworkConfigurations: []*policy.NetworkConfiguration{
					{
						GUID: deviceGUID,
						Name: "DeviceWideNetworkConfig",
						Type: "WiFi",
						WiFi: &policy.WiFi{
							AutoConnect: true,
							Security:    "WPA-EAP",
							SSID:        userApSSID,
							EAP: &policy.EAP{
								Outer:        "PEAP",
								Inner:        "MSCHAPv2",
								Identity:     "testuser",
								Password:     "password",
								UseSystemCAs: false,
							},
						},
					},
				},
			},
		},
	},
	{
		name:          "open",
		sameAp:        false,
		devApOpts:     []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(deviceApSSID)},
		devSecConfFac: nil,
		policy: &policy.DeviceOpenNetworkConfiguration{
			Val: &policy.DeviceOpenNetworkConfigurationValue{
				NetworkConfigurations: []*policy.NetworkConfiguration{
					{
						GUID: deviceGUID,
						Name: "DeviceWideNetworkConfig",
						Type: "WiFi",
						WiFi: &policy.WiFi{
							AutoConnect: true,
							Security:    "None",
							SSID:        deviceApSSID,
						},
					},
				},
			},
		},
	},
	{
		name:      "wpa-psk",
		sameAp:    false,
		devApOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(deviceApSSID)},
		devSecConfFac: wpa.NewConfigFactory(
			"chromeos", wpa.Mode(wpa.ModePureWPA2),
			wpa.Ciphers2(wpa.CipherCCMP),
		),
		policy: &policy.DeviceOpenNetworkConfiguration{
			Val: &policy.DeviceOpenNetworkConfigurationValue{
				NetworkConfigurations: []*policy.NetworkConfiguration{
					{
						GUID: deviceGUID,
						Name: "DeviceWideNetworkConfig",
						Type: "WiFi",
						WiFi: &policy.WiFi{
							AutoConnect: true,
							Security:    "WPA-PSK",
							SSID:        deviceApSSID,
							Passphrase:  "chromeos",
						},
					},
				},
			},
		},
	},
	{
		name:      "wpa-eap",
		sameAp:    false,
		devApOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(deviceApSSID)},
		devSecConfFac: tunneled1x.NewConfigFactory(
			eapCert1.CACred.Cert, eapCert1.ServerCred, eapCert1.CACred.Cert, "testuser", "password",
			tunneled1x.Mode(wpa.ModePureWPA2),
			tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
			tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
		),
		policy: &policy.DeviceOpenNetworkConfiguration{
			Val: &policy.DeviceOpenNetworkConfigurationValue{
				NetworkConfigurations: []*policy.NetworkConfiguration{
					{
						GUID: deviceGUID,
						Name: "DeviceWideNetworkConfig",
						Type: "WiFi",
						WiFi: &policy.WiFi{
							AutoConnect: true,
							Security:    "WPA-EAP",
							SSID:        deviceApSSID,
							EAP: &policy.EAP{
								Outer:        "PEAP",
								Inner:        "MSCHAPv2",
								Identity:     "testuser",
								Password:     "password",
								UseSystemCAs: false,
							},
						},
					},
				},
			},
		},
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PolicyBasic,
		Desc: "Verifies that DUT can connect to the APs with device policy and per-user policy",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.policy.PolicyService"},
		Fixture:      "wificellFixt",
	})
}

func PolicyBasic(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	for _, tc := range testCases {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			s.Log("Configurating device-wide Wi-Fi network")
			deviceAp, err := tf.ConfigureAP(ctx, tc.devApOpts, tc.devSecConfFac)
			if err != nil {
				s.Fatal("Failed to configure device-wide network AP, err: ", err)
			}

			defer func(ctx context.Context, ap *wificell.APIface) {
				if err := tf.DeconfigAP(ctx, ap); err != nil {
					s.Error("Failed to deconfig ap, err: ", err)
				}
			}(ctx, deviceAp)
			ctx, cancel := tf.ReserveForDeconfigAP(ctx, deviceAp)
			defer cancel()

			var userAp = deviceAp
			if !tc.sameAp {
				s.Log("Configurating user Wi-Fi network")
				userAp, err = tf.ConfigureAP(ctx, userApOpts, userSecConfFac)
				if err != nil {
					s.Fatal("Failed to configure user Wi-Fi network, err: ", err)
				}

				defer func(ctx context.Context, ap *wificell.APIface) {
					if err := tf.DeconfigAP(ctx, ap); err != nil {
						s.Error("Failed to deconfig user network ap, err: ", err)
					}
				}(ctx, userAp)
				ctx, cancel = tf.ReserveForDeconfigAP(ctx, userAp)
				defer cancel()
			}

			s.Log("Provision device wide Wi-Fi policy and waiting for DUT to auto connect")
			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			pc := ps.NewPolicyServiceClient(cl.Conn)
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(tc.policy)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Error while marshalling policies to JSON: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
				SkipLogin:  true,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}

			if err := tf.WaitWifiGUIDConnected(ctx, deviceGUID); err != nil {
				s.Fatal("DUT failed to connect to AP with device Wi-Fi policy: ", err)
			}

			if err := tf.VerifyConnection(ctx, deviceAp); err != nil {
				s.Fatal("Failed to verify connection: ", err)
			}

			if _, err = pc.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
				s.Fatal(err, "failed to close policy service chrome instance")
			}

			s.Log("Provision user Wi-Fi policy, login and wait for DUT to auto connect")
			pb = fakedms.NewPolicyBlob()
			pb.AddPolicy(&userNetPolicy)
			pJSON, err = json.Marshal(pb)
			if err != nil {
				s.Fatal("Error while marshalling policies to JSON: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson:     pJSON,
				KeepEnrollment: true,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}
			defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

			if err := tf.WaitWifiGUIDConnected(ctx, userGUID); err != nil {
				s.Fatal("DUT failed to connect to AP with user Wi-Fi policy: ", err)
			}

			if err := tf.VerifyConnection(ctx, userAp); err != nil {
				s.Fatal("Failed to verify connection: ", err)
			}

			s.Log("Deconfigurate")
		})
	}
}

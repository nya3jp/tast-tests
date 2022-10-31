// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/tunneled1x"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

var (
	eapCertPolicyBasic1 = certificate.TestCert1()
	deviceGUID          = "device_network_config"
	userApSSID          = ap.RandomSSID("TAST_TEST_USER_")
	deviceApSSID        = ap.RandomSSID("TAST_TEST_DEVICE_")
)

type policyBasicTestcase struct {
	sameAp        bool
	devApOpts     []ap.Option
	devSecConfFac security.ConfigFactory
	policy        *policy.DeviceOpenNetworkConfiguration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PolicyBasic,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that DUT can connect to the APs with device policy and per-user policy",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.policy.PolicyService"},
		Timeout:      10 * time.Minute,
		Fixture:      "wificellFixtEnrolled",
		Params: []testing.Param{
			{
				// Verify that DUT can connect to a WPA-EAP AP before sign in and still connect to the same WPA-EAP AP after sign in (b/192279295)
				Name: "same_eap_ap",
				Val: policyBasicTestcase{
					sameAp:    true,
					devApOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(userApSSID)},
					devSecConfFac: tunneled1x.NewConfigFactory(
						eapCertPolicyBasic1.CACred.Cert, eapCertPolicyBasic1.ServerCred, eapCertPolicyBasic1.CACred.Cert, "testuser", "password",
						tunneled1x.Mode(wpa.ModePureWPA2),
						tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
						tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
						tunneled1x.Phase2User("tast-user@managedchrome.com", "test0000", tunneled1x.Layer2TypeMSCHAPV2),
					),
					policy: &policy.DeviceOpenNetworkConfiguration{
						Val: &policy.ONC{
							NetworkConfigurations: []*policy.ONCNetworkConfiguration{
								{
									GUID: deviceGUID,
									Name: "DeviceWideNetworkConfig",
									Type: "WiFi",
									WiFi: &policy.ONCWifi{
										AutoConnect: true,
										Security:    "WPA-EAP",
										SSID:        userApSSID,
										EAP: &policy.ONCEap{
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
			},
			{
				// Verify that DUT can connect to an open AP before sign in and switch to a WPA-EAP AP after sign in
				Name: "open",
				Val: policyBasicTestcase{
					sameAp:        false,
					devApOpts:     []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(deviceApSSID)},
					devSecConfFac: nil,
					policy: &policy.DeviceOpenNetworkConfiguration{
						Val: &policy.ONC{
							NetworkConfigurations: []*policy.ONCNetworkConfiguration{
								{
									GUID: deviceGUID,
									Name: "DeviceWideNetworkConfig",
									Type: "WiFi",
									WiFi: &policy.ONCWifi{
										AutoConnect: true,
										Security:    "None",
										SSID:        deviceApSSID,
									},
								},
							},
						},
					},
				},
			},
			{
				// Verify that DUT can connect to a WPA-PSK AP before sign in and switch to a WPA-EAP AP after sign in
				Name: "wpa_psk",
				Val: policyBasicTestcase{
					sameAp:    false,
					devApOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(deviceApSSID)},
					devSecConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModePureWPA2),
						wpa.Ciphers2(wpa.CipherCCMP),
					),
					policy: &policy.DeviceOpenNetworkConfiguration{
						Val: &policy.ONC{
							NetworkConfigurations: []*policy.ONCNetworkConfiguration{
								{
									GUID: deviceGUID,
									Name: "DeviceWideNetworkConfig",
									Type: "WiFi",
									WiFi: &policy.ONCWifi{
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
			},
			{
				// Verify that DUT can connect to a WPA-EAP AP before sign in and switch to a different WPA-EAP AP after sign in
				Name:      "wpa_eap",
				ExtraAttr: []string{"wificell_cq"},
				Val: policyBasicTestcase{
					sameAp:    false,
					devApOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20), ap.SSID(deviceApSSID)},
					devSecConfFac: tunneled1x.NewConfigFactory(
						eapCertPolicyBasic1.CACred.Cert, eapCertPolicyBasic1.ServerCred, eapCertPolicyBasic1.CACred.Cert, "testuser", "password",
						tunneled1x.Mode(wpa.ModePureWPA2),
						tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
						tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
					),
					policy: &policy.DeviceOpenNetworkConfiguration{
						Val: &policy.ONC{
							NetworkConfigurations: []*policy.ONCNetworkConfiguration{
								{
									GUID: deviceGUID,
									Name: "DeviceWideNetworkConfig",
									Type: "WiFi",
									WiFi: &policy.ONCWifi{
										AutoConnect: true,
										Security:    "WPA-EAP",
										SSID:        deviceApSSID,
										EAP: &policy.ONCEap{
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
			},
		},
	})
}

/*
This test verifies if the DUT can connect to specific APs using the enterprise provisioned per-device or per-user policies
by correctly parsing the Open Network Configuration (ONC).

ONC spec: https://chromium.googlesource.com/chromium/src/+/main/components/onc/docs/onc_spec.md

Here are the full steps this test takes:
1. Start two AP BSSes, one to be used in the per-device policy and one to be used in the per-user policy.
2. Provision the per-device and per-user policy to the DUT.
3. Wait for the DUT to connect to the AP provisioned with the device policy and check connection via Ping.
4. Log in the user account, wait for the DUT to connect to the AP provisioned with the user's policy and check connection via Ping.
5. Clean up test, including disconnect, remove SSID entry and deconfig APs.
*/
func PolicyBasic(ctx context.Context, s *testing.State) {
	userGUID := "user_network_config"
	userApOpts := []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.SSID(userApSSID)}
	userSecConfFac := tunneled1x.NewConfigFactory(
		eapCertPolicyBasic1.CACred.Cert, eapCertPolicyBasic1.ServerCred, eapCertPolicyBasic1.CACred.Cert, "tast-user@managedchrome.com", "test0000",
		tunneled1x.Mode(wpa.ModePureWPA2),
		tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
		tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
	)
	userNetPolicy := policy.OpenNetworkConfiguration{
		Val: &policy.ONC{
			NetworkConfigurations: []*policy.ONCNetworkConfiguration{
				{
					GUID: userGUID,
					Name: "UserNetworkConfig",
					Type: "WiFi",
					WiFi: &policy.ONCWifi{
						AutoConnect: true,
						Security:    "WPA-EAP",
						SSID:        userApSSID,
						EAP: &policy.ONCEap{
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

	tc := s.Param().(policyBasicTestcase)
	tf := s.FixtValue().(*wificell.TestFixture)
	pc := ps.NewPolicyServiceClient(tf.RPC().Conn)

	s.Log("Configuring device-wide Wi-Fi network")
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
		s.Log("Configuring user Wi-Fi network")
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

	s.Log("Provision Wi-Fi policy and waiting for DUT to auto connect to device-wide network")
	pb := policy.NewBlob()
	pb.AddPolicy(tc.policy)
	pb.AddPolicy(&userNetPolicy)

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Error while marshalling policies to JSON: ", err)
	}

	if _, err := pc.StartChrome(ctx, &ps.StartChromeRequest{
		PolicyJson:     pJSON,
		KeepEnrollment: true,
		DeferLogin:     true,
	}); err != nil {
		s.Fatal("Failed to start Chrome instance: ", err)
	}
	defer pc.StopChrome(ctx, &empty.Empty{})
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := tf.WaitWifiConnected(ctx, wificell.DefaultDUT, deviceGUID); err != nil {
		s.Fatal("DUT failed to connect to AP with device Wi-Fi policy: ", err)
	}

	defer func(ctx context.Context) {
		req := &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(deviceAp.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for SSID=%s, err: %v", deviceAp.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	if err := tf.VerifyConnection(ctx, deviceAp); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	s.Log("Continue login and wait for DUT to auto connect")
	if _, err = pc.ContinueLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal(err, "failed to login")
	}

	if err := tf.WaitWifiConnected(ctx, wificell.DefaultDUT, userGUID); err != nil {
		s.Fatal("DUT failed to connect to AP with user Wi-Fi policy: ", err)
	}

	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Log("Failed to disconnect WiFi: ", err)
		}
		req := &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(userAp.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for SSID=%s, err: %v", userAp.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, userAp); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}

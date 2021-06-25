// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/diag"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/common/wifi/security/dynamicwep"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

const (
	problemSecurityTypeNone     uint32 = 0
	problemSecurityTypeWep8021x        = 1
	problemSecurityTypeWepPsk          = 2
	problemUnknownSecurityType         = 3
)

type secureWiFiParams struct {
	SecConf  security.ConfigFactory
	Verdict  diag.RoutineVerdict
	Problems []uint32
}

func wep104Keys() []string {
	return []string{
		"0123456789abcdef0123456789", "mlk:ihgfedcba",
		"d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b",
		"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3",
	}
}

func genEAPSecConf() security.ConfigFactory {
	eapCert1 := certificate.TestCert1()
	return dynamicwep.NewConfigFactory(
		eapCert1.CACred.Cert, eapCert1.ServerCred,
		dynamicwep.ClientCACert(eapCert1.CACred.Cert),
		dynamicwep.ClientCred(eapCert1.ClientCred),
		dynamicwep.RekeyPeriod(10))
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagSecureWifi,
		Desc: "Tests that the network diagnostic routine for secure WiFi connection gives correct results with different WiFi security protocols",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"cros-network-health@google.com", // network-health team
		},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.NetDiagService"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		Fixture:      "wificellFixt",
		Params: []testing.Param{{
			Name: "none",
			Val: secureWiFiParams{
				SecConf:  base.NewConfigFactory(),
				Verdict:  diag.VerdictProblem,
				Problems: []uint32{problemSecurityTypeNone},
			},
		}, {
			Name: "wep_psk",
			Val: secureWiFiParams{
				SecConf:  wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
				Verdict:  diag.VerdictProblem,
				Problems: []uint32{problemSecurityTypeWepPsk},
			},
		}, {
			Name: "wep_8021x",
			Val: secureWiFiParams{
				SecConf:  genEAPSecConf(),
				Verdict:  diag.VerdictProblem,
				Problems: []uint32{problemSecurityTypeWep8021x},
			},
		}, {
			Name: "wpa",
			Val: secureWiFiParams{
				SecConf:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				Verdict:  diag.VerdictNoProblem,
				Problems: []uint32{},
			},
		}},
	})
}

// DiagSecureWifi tests that the network diagnostic routine for secure WiFi
// connection returns the correct verdict when the WiFi AP uses a certain
// security protocol.
func DiagSecureWifi(ctx context.Context, s *testing.State) {
	params := s.Param().(secureWiFiParams)

	var apOpts = []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211g),
		hostapd.Channel(1),
		hostapd.SSID(hostapd.RandomSSID("TAST_SECURE_WIFI_")),
	}

	tf := s.FixtValue().(*wificell.TestFixture)
	ap, err := tf.ConfigureAP(ctx, apOpts, params.SecConf)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer tf.DeconfigAP(ctx, ap)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi AP: ", err)
	}
	defer tf.CleanDisconnectWifi(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	di := network.NewNetDiagServiceClient(cl.Conn)

	_, err = di.SetupDiagAPI(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to set up diag API: ", err)
	}

	req := &network.RunRoutineRequest{
		Routine: diag.RoutineSecureWiFiConnection,
	}
	res, err := di.RunRoutine(ctx, req)
	if err != nil {
		s.Fatal("Failed to run diag routine: ", err)
	}

	result := &diag.RoutineResult{
		Verdict:  diag.RoutineVerdict(res.Verdict),
		Problems: res.Problems,
	}
	expectedResult := &diag.RoutineResult{
		Verdict:  params.Verdict,
		Problems: params.Problems,
	}
	if err := diag.CheckRoutineResult(result, expectedResult); err != nil {
		s.Fatal("Routine result did not match: ", err)
	}
}

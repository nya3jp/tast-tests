// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/tethering"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type sapSimpleConnectTestcase struct {
	tetheringOpts []tethering.Option
	pingOps       []ping.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SAPSimpleConnect,
		Desc: "Verifies that DUT can start a Soft AP interface and STAs with different WiFi configurations can connect to the DUT",
		Contacts: []string{
			"jintaolin@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{},
		ServiceDeps:  []string{wificell.TFServiceName},
		HardwareDeps: hwdep.D(hwdep.WifiSAP()),
		Fixture:      "wificellFixtCompanionDut",
		Params: []testing.Param{
			{
				// Verifies that Soft AP DUT can accept connection from a station with no encryption in low band and high band.
				Name: "open",
				Val: []sapSimpleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band2p4g), tethering.NoUplink(true)},
				}, {
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band5g), tethering.NoUplink(true)},
				}},
			},
		},
	})
}

func SAPSimpleConnect(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	testOnce := func(ctx context.Context, s *testing.State, options []tethering.Option, pingOps []ping.Option) {
		tetheringConf, _, err := tf.StartTethering(ctx, options)
		if err != nil {
			s.Error("Failed to start tethering session on DUT, err: ", err)
		}

		defer func(ctx context.Context) {
			if err := tf.StopTethering(ctx); err != nil {
				s.Error("Failed to stop tethering session on DUT, err: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForStopTethering(ctx)
		defer cancel()
		s.Log("Tethering session started")

		_, err = tf.CompanionDUTConnectWifi(ctx, tetheringConf.SSID, dutcfg.ConnSecurity(tetheringConf.SecConf))
		if err != nil {
			s.Error("Failed to connect to Soft AP, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CompanionDUTCleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect from Soft AP, err: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		if err := tf.PingFromCompanionDUT(ctx, pingOps...); err != nil {
			s.Error("Failed to ping from Companion DUT to DUT", err)
		}
	}

	testcases := s.Param().([]sapSimpleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.tetheringOpts, tc.pingOps)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}

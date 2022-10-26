// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/remote/wificell/tethering"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type staConfigOptsType struct {
	disableVHT bool
	disableHT  bool
}

type sapMultipleConnectTestcase struct {
	tetheringOpts []tethering.Option
	staConfigOpts []staConfigOptsType
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SAPMultipleConnect,
		Desc: "Verifies that DUT can start a Soft AP interface and STAs with different WiFi configurations can connect to the DUT",
		Contacts: []string{
			"jintaolin@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell_cross_device", "wificell_cross_device_sap", "wificell_cross_device_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		HardwareDeps: hwdep.D(hwdep.WifiSAP()),
		Fixture:      "wificellFixtCompanionDutWithCapture",
		Params: []testing.Param{
			{
				// Verifies that Soft AP DUT can accept connection from a station with no encryption in low band and high band.
				Name: "band2p4g",
				Val: []sapMultipleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band2p4g), tethering.NoUplink(true)},
					staConfigOpts: []staConfigOptsType{{}, {disableVHT: true}},
				},
				},
			},
			{
				// Verifies that Soft AP DUT can accept connection from a station with no encryption in low band and high band.
				Name: "band5g",
				Val: []sapMultipleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band5g), tethering.NoUplink(true)},
					staConfigOpts: []staConfigOptsType{{}, {disableVHT: true}},
				},
				},
			},
		},
	})
}

func SAPMultipleConnect(ctx context.Context, s *testing.State) {
	/*
		This test checks the soft AP connection of the chromebook by using the following steps:
		1- Configures the main DUT as a soft AP.
		2- Configures the Companion DUT as a STA.
		3- For each backward-compatible specs:
		3a- Connects the the STA to the soft AP.
		3b- Verify the connection by running ping from the STA.
		3c- Deconfigure the STA.
		6- Deconfigure the soft AP.
	*/
	tf := s.FixtValue().(*wificell.TestFixture)
	if tf.NumberOfDUTs() < 2 {
		s.Fatal("Test requires at least 2 DUTs to be declared. Only have ", tf.NumberOfDUTs())
	}

	testOnce := func(ctx context.Context, s *testing.State, options []tethering.Option, staConfigOpts []staConfigOptsType) {
		tetheringConf, _, err := tf.StartTethering(ctx, wificell.DefaultDUT, options, nil)
		if err != nil {
			s.Fatal("Failed to start tethering session on DUT, err: ", err)
		}

		defer func(ctx context.Context) {
			if err := tf.StopTethering(ctx, wificell.DefaultDUT); err != nil {
				s.Error("Failed to stop tethering session on DUT, err: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForStopTethering(ctx)
		defer cancel()
		s.Log("Tethering session started")

		// We want control over capturer start/stop so we don't use fixture with
		// pcap but spawn it here and use manually.
		pcapDevice, ok := tf.Pcap().(support.Capture)
		if !ok {
			s.Fatal("Device without capture support - device type: ", tf.Pcap().RouterType())
		}

		freqOpts := []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Plus)}

		var channel int
		if tetheringConf.Band == tethering.Band2p4g {
			channel = 6
		} else {
			channel = 36
		}

		capturer, err := pcapDevice.StartCapture(ctx, tf.UniqueAPName(), channel, freqOpts)
		if err != nil {
			s.Fatal("Failed to start capturer, err: ", err)
		}
		defer func(ctx context.Context) {
			pcapDevice.StopCapture(ctx, capturer)
		}(ctx)

		cdIdx := wificell.DutIdx(1)
		connectTest := func(ctx context.Context, s *testing.State, options []tethering.Option, staConfigOpts staConfigOptsType) {
			if staConfigOpts.disableVHT {
				if _, err := tf.DUTWifiClient(cdIdx).SetManagerPropertyDisableWiFiVHT(ctx, &wifi.SetManagerPropertyDisableWiFiVHTRequest{Disabled: true}); err != nil {
					s.Error("Failed to set ManagerPropertyDisableWiFiVHT to true: ", err)
				}
				defer func(ctx context.Context) {
					if _, err := tf.DUTWifiClient(cdIdx).SetManagerPropertyDisableWiFiVHT(ctx, &wifi.SetManagerPropertyDisableWiFiVHTRequest{Disabled: false}); err != nil {
						s.Errorf("Failed to set ManagerPropertyDisableWiFiVHT back to false: %v", err)
					}
				}(ctx)
			}
			_, err = tf.ConnectWifiFromDUT(ctx, cdIdx, tetheringConf.SSID)
			if err != nil {
				s.Fatal("Failed to connect to Soft AP, err: ", err)
			}
			defer func(ctx context.Context) {
				if err := tf.DisconnectDUTFromWifi(ctx, cdIdx); err != nil {
					s.Error("Failed to disconnect from Soft AP, err: ", err)
				}
			}(ctx)
			ctx, cancel = tf.ReserveForDisconnect(ctx)
			defer cancel()
			s.Log("Connected")

			addrs, err := tf.DUTIPv4Addrs(ctx, wificell.DefaultDUT)
			if err != nil || len(addrs) == 0 {
				s.Fatal("Failed to get the Soft AP's IP address: ", err)
			}
			if _, err := tf.PingFromSpecificDUT(ctx, cdIdx, addrs[0].String()); err != nil {
				s.Fatal("Failed to ping from Companion DUT to DUT: ", err)
			}

		}
		for _, phyMode := range staConfigOpts {
			connectTest(ctx, s, options, phyMode)
		}
	}

	testcases := s.Param().([]sapMultipleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.tetheringOpts, tc.staConfigOpts)
		}
		s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest)
	}
	s.Log("Tearing down")
}

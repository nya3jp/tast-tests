// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"

	group_owner "chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/remote/network/iperf"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: P2PPerf,
		Desc: "Tests P2P performance between two chromebooks",
		Contacts: []string{
			"arowa@google.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell_cross_device", "wificell_cross_device_p2p", "wificell_cross_device_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtCompanionDut",
	})
}

func P2PPerf(ctx context.Context, s *testing.State) {
	/*
		This test checks the p2p connection between two chromebooks by using
		the following steps:
		1- Configure the main DUT as a p2p group owner (GO).
		2- Configure the Companion DUT as a p2p client.
		3- Connect the the p2p client to the GO network.
		4- Route the IP address in both GO and client.
		5- Run Iperf TCP.
		6- Delete the IP route created in step 4.
		7- Deconfigure the p2p client.
		8- Deconfigure the p2p GO.
	*/
	tf := s.FixtValue().(*wificell.TestFixture)
	if err := tf.P2PConfigureGO(ctx, wificell.P2PDeviceDUT, group_owner.SetP2PGOMode(group_owner.PhyModeHT40), group_owner.SetP2PGOFreq(5180)); err != nil {
		s.Fatal("Failed to configure the p2p group owner (GO): ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.P2PDeconfigureGO(ctx); err != nil {
			s.Error("Failed to deconfigure the p2p group owner (GO): ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigP2P(ctx)
	defer cancel()

	if err := tf.P2PConfigureClient(ctx, wificell.P2PDeviceCompanionDUT); err != nil {
		s.Fatal("Failed to configure the p2p client: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.P2PDeconfigureClient(ctx); err != nil {
			s.Error("Failed to deconfigure the p2p client: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigP2P(ctx)
	defer cancel()

	if err := tf.P2PConnect(ctx); err != nil {
		s.Fatal("Failed to connect the p2p client to the p2p group owner (GO) network: ", err)
	}

	if err := tf.P2PAddIPRoute(ctx); err != nil {
		s.Fatal("Failed to route the IP addresses in the p2p group owner (GO) and the p2p client: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.P2PDeleteIPRoute(ctx); err != nil {
			s.Error("Failed to delete the IP routing in the p2p group owner and p2p client: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeleteIPRoute(ctx)
	defer cancel()

	// Print the P2P channel configuration.
	iwr := iw.NewRemoteRunner(tf.P2PGOConn())
	chConfig, err := iwr.RadioConfig(ctx, tf.P2PGOIface())
	if err != nil {
		s.Error("Failed the P2P channel configuration: ", err)
	}

	testing.ContextLogf(ctx, "P2P channel configuration: Channel Number = %d, Frequency = %d, Width = %d", chConfig.Number, chConfig.Freq, chConfig.Width)

	finalResult, err := tf.P2PPerf(ctx)
	if err != nil {
		s.Fatal("Failed to run performance test: ", err)
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	pv.Set(perf.Metric{
		Name:      "p2p_tcp_ave_tput_ch" + strconv.Itoa(chConfig.Number) + "_mode_" + string(group_owner.PhyModeHT40),
		Unit:      "Mbps",
		Direction: perf.BiggerIsBetter,
	}, float64(finalResult.Throughput/iperf.Mbps))
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket/layers"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        WakeOnWifiNetDetect,
		Desc:        "Verifies that the DUT can resume from suspend with WoWLAN net detect feature",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func WakeOnWifiNetDetect(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	// Set up WoWLAN.
	const ssid = "TAST_WoWLAN"
	const channel = 1
	const interval = 5000
	dut := s.DUT()
	freq, err := hostapd.ChannelToFrequency(channel)
	if err != nil {
		s.Fatalf("Failed to get frequency of channel %d: %v", channel, err)
	}
	// TODO: Get current phy instead of hardcode.
	const phyName = "phy0"
	// TODO: Move this into iw.Runner.
	if err := dut.Command("iw", phyName, "wowlan", "enable",
		"net-detect", "interval", strconv.Itoa(interval),
		"matches", "ssid", ssid, "freqs", strconv.Itoa(freq),
		"randomise",
	).Run(ctx); err != nil {
		s.Fatal("Failed to set up WoWLAN: ", err)
	}
	defer func(ctx context.Context) {
		if err := dut.Command("iw", phyName, "wowlan", "disable").Run(ctx); err != nil {
			s.Error("Failed to restore wowlan to disable: ", err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// Spawning he routine to suspend DUT and wait.
	done := make(chan error, 1)
	// Wait the bgroutine to exit.
	defer func() { <-done }()
	bgCtx, cancel := context.WithCancel(ctx)
	// Close context to notify the routine to end.
	defer cancel()
	go func(ctx context.Context) {
		defer close(done)
		done <- func() error {
			start := time.Now()
			const suspendSecs = 100

			s.Log("Suspending DUT")
			out, err := s.DUT().Command("powerd_dbus_suspend", "--print_wakeup_type",
				fmt.Sprintf("--suspend_for_sec=%d", suspendSecs)).CombinedOutput(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to run power_dbus_suspend")
			}
			timeDiff := time.Since(start)
			s.Log("DUT is suspended for: ", timeDiff)
			s.Log("Wake reason: ", strings.TrimSpace(string(out)))
			// TODO: check the reason of wake.
			return nil
		}()
		// TODO: How can we recover if suspend does not return? maybe servo?
	}(bgCtx)

	s.Log("Wait until DUT is suspended")
	if err := dut.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT suspend: ", err)
	}

	s.Log("DUT suspended, configuring AP to wake it up")
	capturer, err := func(ctx context.Context) (ret *pcap.Capturer, retErr error) {
		apOps := []hostapd.Option{hostapd.SSID(ssid), hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(channel)}
		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure AP")
		}
		defer func(ctx context.Context) {
			s.Log("Deconfiguring AP")
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				if retErr == nil {
					ret = nil
					retErr = errors.Wrap(err, "failed to deconfig ap")
				} else {
					s.Log("Failed to deconfig ap: ", err)
				}
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		s.Log("AP configured, waiting for DUT to wake up")
		if err := <-done; err != nil {
			s.Fatal("Failed to wait for DUT to wake up: ", err)
		}
		capturer, ok := tf.Capturer(ap)
		if !ok {
			return nil, errors.New("no capturer spawned")
		}
		return capturer, nil
	}(ctx)

	pcapPath, err := capturer.PacketPath(ctx)
	if err != nil {
		s.Fatal("Failed to get pcap file: ", err)
	}

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT")
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface")
	}

	// Verify MAC randomization.
	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(layers.LayerTypeDot11MgmtProbeReq, nil),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}
	if len(packets) == 0 {
		s.Fatal("No probe request found in pcap")
	}
	s.Logf("Total %d probe requests found", len(packets))
	for _, p := range packets {
		// Get sender address.
		layer := p.Layer(layers.LayerTypeDot11)
		if layer == nil {
			s.Fatalf("ProbeReq packet %v does not have Dot11 layer", p)
		}
		dot11, ok := layer.(*layers.Dot11)
		if !ok {
			s.Fatalf("Dot11 layer output %v not *layers.Dot11", p)
		}
		sender := dot11.Address2

		// Verify sender address.
		if bytes.Equal(sender, mac) {
			s.Fatal("Expect randomized MAC but found probe request with hardware MAC")
		}
	}
}

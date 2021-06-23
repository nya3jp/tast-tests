// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MalformedProbeResp,
		Desc: "Test that we can stay connected to the configured AP when receiving malformed probe responses from an AP that we are not connected to",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
	})
}

func MalformedProbeResp(ctx context.Context, s *testing.State) {
	// The test uses framesender to periodically send malformed probe
	// responses and trigger background scans on the same channel. Then
	// it verifies that the SSID of the malformed responses are found
	// and no disconnection during the period.
	tf := s.FixtValue().(*wificell.TestFixture)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	// We'll use `iw scan` to trigger background scan, turn it off
	// in shill so we won't race with shill on the device.
	ctx, restoreBgscan, err := tf.TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off background scan: ", err)
	}
	defer func() {
		if err := restoreBgscan(); err != nil {
			s.Error("Failed to restore background scan config: ", err)
		}
	}()

	s.Log("Configuring AP")
	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("Deconfiguring AP")
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	// Get frequency for later scan.
	freq, err := hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		s.Fatal("Failed to get frequency of the AP: ", err)
	}

	s.Log("Connecting")
	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("Disconnecting")
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	// Get DUT interface name and MAC address.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get client interface")
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface")
	}

	// Start the background sender of malformed probe response.
	sender, err := legacyRouter.NewFrameSender(ctx, ap.Interface())
	if err != nil {
		s.Fatal("Failed to create frame sender: ", err)
	}
	defer func(ctx context.Context) {
		if err := legacyRouter.CloseFrameSender(ctx, sender); err != nil {
			s.Error("Failed to close frame sender: ", err)
		}
	}(ctx)
	ctx, cancel = legacyRouter.ReserveForCloseFrameSender(ctx)
	defer cancel()

	// Set up background frame sender sending malformed probe response.
	const ssidPrefix = "TestingProbes"
	fsOps := []framesender.Option{
		framesender.SSIDPrefix(ssidPrefix),
		framesender.NumBSS(1),
		framesender.Count(0),  // Infinite run.
		framesender.Delay(50), // 50ms delay.
		framesender.DestMAC(mac.String()),
		// Append a vendor specific IE (0xdd) with broken length (0xb7 > the
		// length of remaining payload) and OUI=00:1a:11 which is Google.
		framesender.ProbeRespFooter([]byte("\xdd\xb7\x00\x1a\x11\x01\x01\x02\x03")),
	}
	sender.Start(ctx, framesender.TypeProbeResponse, ap.Config().Channel, fsOps...)
	defer func(ctx context.Context) {
		if err := sender.Stop(ctx); err != nil {
			s.Error("Failed to stop frame sender: ", err)
		}
	}(ctx)
	ctx, cancel = sender.ReserveForStop(ctx)
	defer cancel()

	runOnce := func(ctx context.Context) (retErr error) {
		const scanLoopTime = 60 * time.Second
		const scanLoopInterval = 10 * time.Second

		start := time.Now()
		received := 0
		iwr := iw.NewRemoteRunner(s.DUT().Conn())
		for round := 1; time.Since(start) < scanLoopTime; round++ {
			s.Logf("Scan %d", round)
			result, err := iwr.TimedScan(ctx, iface, []int{freq}, nil)
			if err != nil {
				return errors.Wrap(err, "failed to scan")
			}
			for _, bss := range result.BSSList {
				s.Log("Found BSS: ", bss.SSID)
				if strings.HasPrefix(bss.SSID, ssidPrefix) {
					received++
				}
			}
			if err := testing.Sleep(ctx, scanLoopInterval); err != nil {
				return err
			}
		}
		if received == 0 {
			return errors.New("no probe response received")
		}

		return nil
	}

	if err := tf.AssertNoDisconnect(ctx, runOnce); err != nil {
		s.Fatal("Scan failed: ", err)
	}
}

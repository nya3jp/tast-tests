// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"net"
	"time"

	cip "chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MARSSIDRoam,
		Desc: "Tests MAC Address randomization during roam between APs of different SSIDs",
		Contacts: []string{
			"jck@semihalf.com",                // Author.
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func MARSSIDRoam(ctx context.Context, s *testing.State) {
	// Goal of this test is to verify if MAC address gets updated when we move between different SSIDs.
	// It's not roaming in the full sense, but relies on the similar mechanics as roaming.
	// Steps:
	// * Configure AP1/SSID1,
	// * Connect to AP1,
	// * Check MAC Address (MAC-AP1),
	// * Configure AP2/SSID2,
	// * Connect to AP2,
	// * Check MAC Address (MAC-AP2),
	// * Unconfigure AP2,
	// * Wait until MAC changes to MAC-AP1,
	// * Verify MAC-AP2 is no longer used while sending traffic.

	tf := s.FixtValue().(*wificell.TestFixture)

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	hwMAC, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("Read HW MAC: ", hwMAC)
	defer func(ctx context.Context, iface string, mac net.HardwareAddr) {
		if err := ipr.SetLinkDown(ctx, iface); err != nil {
			s.Error("Failed to set the interface down: ", err)
		}
		if err := ipr.SetMAC(ctx, iface, mac); err != nil {
			s.Error("Failed to revert the original MAC: ", err)
		}
		if err := ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}(ctx, iface, hwMAC)
	// Make sure the device is up.
	link, err := ipr.State(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get link state")
	}
	if link != cip.LinkStateUp {
		if err := ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}

	const (
		ap1Channel = 48
		ap2Channel = 1
	)

	// Generate BSSIDs for the two APs.
	mac1, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	mac2, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	ap1BSSID := mac1.String()
	ap2BSSID := mac2.String()

	// Configure the initial AP.
	optionsAP1 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap1Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap1BSSID)}
	ap1, err := tf.ConfigureAP(ctx, optionsAP1, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	// Connect with PersistentRandom policy.
	configProps := map[string]interface{}{
		shillconst.ServicePropertyWiFiRandomMACPolicy: shillconst.MacPolicyPersistentRandom,
	}

	// We want control over capturer start/stop so we don't use fixture with
	// pcap but spawn it here and use manually.
	pcapDevice, ok := tf.Pcap().(support.Capture)
	if !ok {
		s.Fatal("Device without capture support - device type: ", tf.Pcap().RouterType())
	}

	freqOpts, err := ap1.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get Freq Opts: ", err)
	}

	pcapPath, err := wifiutil.CollectPcapForAction(ctx, pcapDevice, "connect", ap1.Config().Channel, freqOpts,
		func(ctx context.Context) error {
			_, err := tf.ConnectWifiAP(ctx, ap1, dutcfg.ConnProperties(configProps))
			if err != nil {
				return errors.Wrap(err, "DUT: failed to connect to WiFi")
			}
			return nil
		})
	if err != nil {
		s.Fatal("Failed to collect pcap or perform action: ", err)
	}

	connMAC, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}

	s.Log("MAC after connection: ", connMAC)
	if err := wifiutil.VerifyMACIsChanged(ctx, connMAC, pcapPath, []net.HardwareAddr{hwMAC}); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

	roamSucceeded := false
	defer func(ctx context.Context) {
		if !roamSucceeded {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	// Set up the second AP interface on the same device with the same
	// SSID, but on different band (5 GHz for AP1 and 2.4 GHz for AP2).
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap2Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap2BSSID)}
	ap2, err := tf.ConfigureAP(ctx, optionsAP2, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	deconfigured := false
	defer func(ctx context.Context) {
		if !deconfigured {
			if err := tf.DeconfigAP(ctx, ap2); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
	defer cancel()

	freqOpts, err = ap2.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get Freq Opts: ", err)
	}

	pcapPath, err = wifiutil.CollectPcapForAction(ctx, pcapDevice, "connect2", ap2.Config().Channel, freqOpts,
		func(ctx context.Context) error {
			_, err := tf.ConnectWifiAP(ctx, ap2, dutcfg.ConnProperties(configProps))
			if err != nil {
				return errors.Wrap(err, "DUT: failed to connect to WiFi")
			}
			return nil
		})
	if err != nil {
		s.Fatal("Failed to collect pcap or perform action: ", err)
	}

	connMAC2, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}

	s.Log("MAC after 2nd connection: ", connMAC2)
	if err := wifiutil.VerifyMACIsChanged(ctx, connMAC2, pcapPath, []net.HardwareAddr{hwMAC}); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

	roamSucceeded = true
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)

	// Trigger roaming.
	if err := tf.DeconfigAP(ctx, ap2); err != nil {
		s.Error("Failed to deconfig the AP: ", err)
	}
	deconfigured = true

	if err = testing.Poll(ctx, func(ctx context.Context) error {
		roamMAC, err := ipr.MAC(ctx, iface)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get MAC of WiFi interface"))
		}
		if bytes.Equal(roamMAC, connMAC) {
			return nil
		}
		testing.Sleep(ctx, time.Second)
		return errors.Errorf("current MAC is %s", roamMAC)
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed change MAC address of WiFi interface: ", err)
	}
	s.Log("DUT: roamed")

	roamMAC, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("MAC after forced roaming: ", roamMAC)

	pcapPath, err = wifiutil.CollectPcapForAction(ctx, pcapDevice, "verify", ap1.Config().Channel, freqOpts,
		func(ctx context.Context) error {
			if err := tf.VerifyConnection(ctx, ap1); err != nil {
				return errors.Wrap(err, "DUT: failed to verify connection")
			}
			return nil
		})
	if err != nil {
		s.Fatal("Failed to collect pcap or perform action: ", err)
	}

	if err := wifiutil.VerifyMACIsChanged(ctx, roamMAC, pcapPath, []net.HardwareAddr{hwMAC, connMAC2}); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

}

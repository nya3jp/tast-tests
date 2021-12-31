// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	cip "chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MARBSSIDRoam,
		Desc: "Tests MAC Address randomization during roam between APs of the same SSID",
		Contacts: []string{
			"jck@semihalf.com",                // Author.
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func MARBSSIDRoam(ctx context.Context, s *testing.State) {
	// This test is essentially RoamDbus test enhanced to enable MAC Address Randomization
	// and verify its behavior.
	tf := s.FixtValue().(*wificell.TestFixture)

	allowRoamResp, err := tf.WifiClient().GetScanAllowRoamProperty(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get the ScanAllowRoam property: ", err)
	}
	if allowRoamResp.Allow {
		if _, err := tf.WifiClient().SetScanAllowRoamProperty(ctx, &wifi.SetScanAllowRoamPropertyRequest{Allow: false}); err != nil {
			s.Error("Failed to set ScanAllowRoam property to false: ", err)
		}
		defer func(ctx context.Context) {
			if _, err := tf.WifiClient().SetScanAllowRoamProperty(ctx, &wifi.SetScanAllowRoamPropertyRequest{Allow: allowRoamResp.Allow}); err != nil {
				s.Errorf("Failed to set ScanAllowRoam property back to %v: %v", allowRoamResp.Allow, err)
			}
		}(ctx)
	}

	ctx, restoreBg, err := tf.WifiClient().TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off the background scan: ", err)
	}
	defer func() {
		if err := restoreBg(); err != nil {
			s.Error("Failed to restore the background scan config: ", err)
		}
	}()

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	hwMac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("Read HW MAC: ", hwMac)
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
	}(ctx, iface, hwMac)
	// Make sure the device is up
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

	ap1SSID := ap1.Config().SSID

	// Connect with PersistentRandom policy.
	configProps := map[string]interface{}{
		shillconst.ServicePropertyWiFiRandomMACPolicy: shillconst.MacPolicyPersistentRandom,
	}

	// We want control over capturer start/stop so we don't use fixture with
	// pcap but spawn it here and use manually.
	pcapDevice, ok := tf.Pcap().(router.SupportCapture)
	if !ok {
		s.Fatal("Device without capture support - device type: ", tf.Pcap().RouterType())
	}

	freqOpts, err := ap1.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("failed to get Freq Opts", err)
	}

	var servicePath string
	pcapPath, err := wifiutil.CollectPcapForAction(ctx, pcapDevice, "connect", ap1.Config().Channel, freqOpts,
		func(ctx context.Context) error {
			if resp, err := tf.ConnectWifiAP(ctx, ap1, dutcfg.ConnProperties(configProps)); err != nil {
				s.Fatal("DUT: failed to connect to WiFi: ", err)
			} else {
				servicePath = resp.ServicePath
			}
			return nil
		})
	if err != nil {
		s.Fatal("Failed to collect pcap or perform action: ", err)
	}

	connMac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("failed to get MAC of WiFi interface: ", err)
	}

	s.Log("MAC after connection: ", connMac)
	if err := verifyMACIsChanged(ctx, connMac, pcapPath, []net.HardwareAddr{hwMac}); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

	roamSucceeded := false
	defer func(ctx context.Context) {
		if roamSucceeded {
			return
		}
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	props := []*wificell.ShillProperty{{
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateConfiguration},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateReady},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateIdle},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{ap2BSSID},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
	}}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	monitorProps := []string{shillconst.ServicePropertyIsConnected}
	waitForProps, err := tf.WifiClient().ExpectShillProperty(waitCtx, servicePath, props, monitorProps)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	// Set up the second AP interface on the same device with the same
	// SSID, but on different band (5 GHz for AP1 and 2.4 GHz for AP2).
	optionsAP2 := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(ap2Channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ap1SSID), hostapd.BSSID(ap2BSSID)}
	ap2, err := tf.ConfigureAP(ctx, optionsAP2, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap2); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
	defer cancel()

	discCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := tf.WifiClient().DiscoverBSSID(discCtx, ap2BSSID, iface, []byte(ap1SSID)); err != nil {
		s.Fatalf("DUT: failed to find the BSSID %s: %v", ap2BSSID, err)
	}

	// Send roam command to shill, and shill will send D-Bus roam command to wpa_supplicant.
	s.Logf("Requesting roam from %s to %s", ap1BSSID, ap2BSSID)
	if err := tf.WifiClient().RequestRoam(ctx, iface, ap2BSSID, 30*time.Second); err != nil {
		s.Errorf("DUT: failed to roam from %s to %s: %v", ap1BSSID, ap2BSSID, err)
	}

	monitorResult, err := waitForProps()
	if err != nil {
		s.Fatal("DUT: failed to wait for the properties, err: ", err)
	}
	s.Log("DUT: roamed")
	roamSucceeded = true
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)

	// Assert there was no disconnection during roaming.
	for _, ph := range monitorResult {
		if ph.Name == shillconst.ServicePropertyIsConnected {
			if !ph.Value.(bool) {
				s.Fatal("DUT: failed to stay connected during the roaming process")
			}
		}
	}

	roamMac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("failed to get MAC of WiFi interface", err)
	}

	freqOpts, err = ap2.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("failed to get Freq Opts", err)
	}

	pcapPath, err = wifiutil.CollectPcapForAction(ctx, pcapDevice, "connect", ap2.Config().Channel, freqOpts,
		func(ctx context.Context) error {
			if err := tf.VerifyConnection(ctx, ap2); err != nil {
				s.Fatal("DUT: failed to verify connection: ", err)
			}
			return nil
		})
	if err != nil {
		s.Fatal("Failed to collect pcap or perform action: ", err)
	}

	s.Log("MAC after roaming: ", roamMac)
	if err := verifyMACIsKept(ctx, roamMac, pcapPath, connMac); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

}

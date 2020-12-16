// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RoamDbus,
		Desc:     "Tests an intentional client-driven roam between APs",
		Contacts: []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:     []string{"group:wificell", "wificell_func", "wificell_unstable"},
		// These boards have Broadcom WiFi chip which doesn't support supplicant-based roaming.
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform("veyron_mickey"), hwdep.SkipOnPlatform("veyron_speedy")),
		ServiceDeps:  []string{wificell.TFServiceName},
		Pre:          wificell.TestFixturePre(),
		Vars:         []string{"router", "pcap"},
	})
}

func RoamDbus(ctx context.Context, s *testing.State) {
	// This test seeks to associate the DUT with an AP with a set of
	// association parameters. Then it creates a second AP with a different
	// set of association parameters but the same SSID, and sends roam
	// command to shill. After receiving the roam command, shill sends a D-Bus
	// roam command to wpa_supplicant. The test expects that the DUT
	// successfully connects to the second AP within a reasonable amount of time.
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

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
	mac := []net.HardwareAddr{mac1, mac2}

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
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	ap1SSID := ap1.Config().SSID

	// Connect to the initial AP.
	var servicePath string
	if resp, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	} else {
		servicePath = resp.ServicePath
		s.Logf("Service path: %s", servicePath)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("DUT: failed to verify connection: ", err)
	}

	pingF := func() (wificell.ResultType, error) {
		pr := remoteping.NewRemoteRunner(s.DUT().Conn())
		//ap0.ServerIP().String()
		res, err := pr.Ping(ctx, ap1.ServerIP().String(), ping.Count(100))
		if err != nil {
			testing.ContextLog(ctx, "ping error, ", err)
			return wificell.ResultType{}, err
		}
		testing.ContextLogf(ctx, "ping statistics=%+v", res)

		return wificell.ResultType{Data: res.Loss, Timestamp: time.Now()}, nil
	}
	vf := wificell.NewVerifier(ctx, pingF)

	props := [][]*wificell.ShillProperty{{
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{ap2BSSID},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		},
	}, {
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{ap1BSSID},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		},
	}}
	vf.StartJob()

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

	ap := []*wificell.APIface{ap1, ap2}
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: ", err)
	}
	for i := 0; i < 10; i++ {
		monitorProps := []string{shillconst.ServicePropertyIsConnected}
		waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		currService, _ := tf.WifiService(ctx)
		waitForProps, err := tf.ExpectShillProperty(waitCtx, currService.GetServicePath(), props[i%2], monitorProps)
		if err != nil {
			s.Fatal("DUT: failed to create a property watcher, err: ", err)
		}
		discCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := tf.DiscoverBSSID(discCtx, mac[(i+1)%2].String(), iface, []byte(ap1SSID)); err != nil {
			s.Fatalf("DUT: failed to find the BSSID %s: %v", mac[(i+1)%2].String(), err)
		}

		// Send roam command to shill, and shill will send D-Bus roam command to wpa_supplicant.
		s.Logf("Step %d. Requesting roam from %s to %s", i+1, mac[i%2].String(), mac[(i+1)%2].String())
		if err := tf.RequestRoam(ctx, iface, mac[(i+1)%2].String(), 30*time.Second); err != nil {
			s.Errorf("DUT: failed to roam from %s to %s: %v", mac[i%2].String(), mac[(i+1)%2].String(), err)
		}

		monitorResult, err := waitForProps()
		if err != nil {
			s.Fatal("DUT: failed to wait for the properties, err: ", err)
		}
		s.Log("DUT: roamed")

		// Assert there was no disconnection during roaming.
		for _, ph := range monitorResult {
			if ph.Name == shillconst.ServicePropertyIsConnected {
				if !ph.Value.(bool) {
					s.Fatal("DUT: failed to stay connected during the roaming process")
				}
			}
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return tf.VerifyConnection(ctx, ap[(i+1)%2])
		}, &testing.PollOptions{
			Timeout:  20 * time.Second,
			Interval: time.Second,
		}); err != nil {
			s.Fatal("Failed to verify connection: ", err)
		}
	}
	verificatonResults(ctx, vf)
}

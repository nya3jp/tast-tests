// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PMKSACaching,
		Desc:        "Verifies 802.1x authentication is bypassed and uses PMKSA caching instead when a cache candidate is available",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

func PMKSACaching(ctx context.Context, s *testing.State) {
	const (
		ap0Channel  = 1
		ap0BSSID    = "00:11:22:33:44:55"
		ap1Channel  = 44
		ap1BSSID    = "00:11:22:33:44:56"
		roamTimeout = 15 * time.Second
	)

	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	cert := certificate.TestCert1()
	secConfFac := wpaeap.NewConfigFactory(
		cert.CACert, cert.ServerCred,
		wpaeap.ClientCACert(cert.CACert),
		wpaeap.ClientCred(cert.ClientCred),
		// PMKSA caching is only defined for WPA2.
		wpaeap.Mode(wpa.ModePureWPA2),
	)
	ssid := hostapd.RandomSSID("TAST_TEST_")

	configureAP := func(ctx context.Context, channel int, bssid string, ap **wificell.APIface) (context.Context, func()) {
		apOps := []hostapd.Option{
			hostapd.SSID(ssid), hostapd.BSSID(bssid), hostapd.Mode(hostapd.Mode80211nPure),
			hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20),
		}
		var err error
		*ap, err = tf.ConfigureAP(ctx, apOps, secConfFac)
		if err != nil {
			s.Fatal("Failed to configure AP: ", err)
		}
		sCtx, cancel := tf.ReserveForDeconfigAP(ctx, *ap)
		return sCtx, func() {
			cancel()
			if *ap == nil {
				return
			}
			if err := tf.DeconfigAP(ctx, *ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
		}
	}

	var ap0 *wificell.APIface
	ctx, cancel = configureAP(ctx, ap0Channel, ap0BSSID, &ap0)
	defer cancel()
	s.Log("AP0 setup done; connecting")

	connResp, err := tf.ConnectWifiAP(ctx, ap0)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.PingFromDUT(ctx, ap0.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	var ap1 *wificell.APIface
	ctx, cancel = configureAP(ctx, ap1Channel, ap1BSSID, &ap1)
	defer cancel()

	roamToAp1Props := []*wificell.ShillProperty{{
		Property:         shillconst.ServicePropertyState,
		ExpectedValues:   []interface{}{shillconst.ServiceStateConfiguration},
		UnexpectedValues: []interface{}{shillconst.ServiceStateIdle},
		Method:           network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:         shillconst.ServicePropertyState,
		ExpectedValues:   shillconst.ServiceConnectedStates,
		UnexpectedValues: []interface{}{shillconst.ServiceStateIdle},
		Method:           network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{ap1BSSID},
		Method:         network.ExpectShillPropertyRequest_CHECK_ONLY,
	}}
	wCtx, cancel := context.WithTimeout(ctx, roamTimeout)
	defer cancel()
	waitForRoam, err := tf.ExpectShillProperty(wCtx, connResp.ServicePath, roamToAp1Props)
	if err != nil {
		s.Fatal("Failed to create a property watcher on DUT: ", err)
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get interface from DUT: ", err)
	}
	disCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := tf.DiscoverBSSID(disCtx, ap1BSSID, iface, []byte(ssid)); err != nil {
		s.Fatal("Failed to discover AP1's BSSID: ", err)
	}
	// Send roam command to shill, and shill will send D-Bus roam command to wpa_supplicant.
	s.Logf("Requesting roam from %s to %s", ap0BSSID, ap1BSSID)
	if err := tf.RequestRoam(ctx, iface, ap1BSSID, 30*time.Second); err != nil {
		s.Errorf("Failed to roam from %s to %s: %v", ap0BSSID, ap1BSSID, err)
	}

	s.Log("Waiting for roaming to AP1")
	if err := waitForRoam(); err != nil {
		s.Fatal("Failed to wait for roaming to AP1: ", err)
	}

	roamToAp0Props := []*wificell.ShillProperty{{
		Property:       shillconst.ServicePropertyState,
		ExpectedValues: []interface{}{shillconst.ServiceStateConfiguration},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyState,
		ExpectedValues: shillconst.ServiceConnectedStates,
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{ap0BSSID},
		Method:         network.ExpectShillPropertyRequest_CHECK_ONLY,
	}}
	wCtx, cancel = context.WithTimeout(ctx, roamTimeout)
	defer cancel()
	waitForRoam, err = tf.ExpectShillProperty(wCtx, connResp.ServicePath, roamToAp0Props)
	if err != nil {
		s.Fatal("Failed to create a property watcher on DUT: ", err)
	}

	if err := tf.DeconfigAP(ctx, ap1); err != nil {
		s.Fatal("Failed to deconfig AP: ", err)
	}
	ap1 = nil

	s.Log("Waiting for falling back to AP0")
	if err := waitForRoam(); err != nil {
		s.Fatal("Failed to wait for falling back to AP0: ", err)
	}

	if err := tf.VerifyConnection(ctx, ap0); err != nil {
		s.Error("Failed to verify connection to AP0 after fallbacking: ", err)
	}

	if err := ap0.ConfirmPMKSACached(ctx); err != nil {
		s.Fatal("Failed to confirm PMKSA cached: ", err)
	}
}

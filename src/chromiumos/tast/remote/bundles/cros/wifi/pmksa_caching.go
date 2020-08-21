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
		Desc:        "Verifies that 802.1x authentication (EAP exchange) is bypassed and PMKSA is done using PMK caching when it is available",
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
		roamTimeout = 30 * time.Second
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

	configureAP := func(ctx context.Context, channel int, bssid string) (*wificell.APIface, error) {
		apOps := []hostapd.Option{
			hostapd.SSID(ssid), hostapd.BSSID(bssid), hostapd.Mode(hostapd.Mode80211nPure),
			hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20),
		}
		return tf.ConfigureAP(ctx, apOps, secConfFac)
	}

	roamProps := func(bssid string, unexpectedIdle bool) []*wificell.ShillProperty {
		props := []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: []interface{}{shillconst.ServiceStateConfiguration},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		}, {
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: shillconst.ServiceConnectedStates,
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		}, {
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{bssid},
			Method:         network.ExpectShillPropertyRequest_CHECK_ONLY,
		}}
		if unexpectedIdle {
			props[0].UnexpectedValues = []interface{}{shillconst.ServiceStateIdle}
			props[1].UnexpectedValues = []interface{}{shillconst.ServiceStateIdle}
		}
		return props
	}

	ap0, err := configureAP(ctx, ap0Channel, ap0BSSID)
	if err != nil {
		s.Fatal("Failed to configure AP0: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap0); err != nil {
			s.Error("Failed to deconfig AP0: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap0)
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

	// ap1Ctx should only be used by configure/deconfigure AP1.
	ap1Ctx := ctx
	// Reserve time for deconfig ap1. Note that ap1 should be created after the waitForRoam property watcher, we borrow ap0 to reserve time for DeconfigAP.
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap0)
	defer cancel()

	roamToAp1Props := roamProps(ap1BSSID, true)
	roamCtx, cancel := context.WithTimeout(ctx, roamTimeout)
	defer cancel()
	waitForRoam, err := tf.ExpectShillProperty(roamCtx, connResp.ServicePath, roamToAp1Props)
	if err != nil {
		s.Fatal("Failed to create a property watcher on DUT: ", err)
	}
	skippedRecver, err := tf.EAPAuthSkipped(roamCtx)
	if err != nil {
		s.Fatal("Failed to create a EAP authentication watcher: ", err)
	}

	// Configure AP1 after ExpectShillProperty() because a roaming may happen automatically right after AP1 is up.
	ap1, err := configureAP(ap1Ctx, ap1Channel, ap1BSSID)
	if err != nil {
		s.Fatal("Failed to configure AP1: ", err)
	}
	defer func(ctx context.Context) {
		if ap1 == nil {
			// AP1 is already closed.
			return
		}
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig AP1: ", err)
		}
	}(ap1Ctx)

	iface, err := tf.ClientInterface(roamCtx)
	if err != nil {
		s.Fatal("Failed to get interface from DUT: ", err)
	}
	if err := tf.DiscoverBSSID(roamCtx, ap1BSSID, iface, []byte(ssid)); err != nil {
		s.Fatal("Failed to discover AP1's BSSID: ", err)
	}
	if err := tf.RequestRoam(roamCtx, iface, ap1BSSID, 5*time.Second); err != nil {
		s.Errorf("Failed to roam from %s to %s: %v", ap0BSSID, ap1BSSID, err)
	}

	s.Log("Waiting for roaming to AP1")
	if err := waitForRoam(); err != nil {
		s.Fatal("Failed to wait for roaming to AP1: ", err)
	}
	skipped, err := skippedRecver()
	if err != nil {
		s.Fatal("Failed to wait for confirming skipping EAP authentication: ", err)
	}
	if skipped {
		s.Error("EAP authentication is skipped")
	} else {
		s.Log("EAP authentication is not skipped as expected")
	}

	roamToAp0Props := roamProps(ap0BSSID, false)
	roamCtx, cancel = context.WithTimeout(ctx, roamTimeout)
	defer cancel()
	waitForRoam, err = tf.ExpectShillProperty(roamCtx, connResp.ServicePath, roamToAp0Props)
	if err != nil {
		s.Fatal("Failed to create a property watcher on DUT: ", err)
	}
	skippedRecver, err = tf.EAPAuthSkipped(roamCtx)
	if err != nil {
		s.Fatal("Failed to create a EAP authentication watcher: ", err)
	}

	if err := tf.DeconfigAP(roamCtx, ap1); err != nil {
		s.Fatal("Failed to deconfig AP: ", err)
	}
	ap1 = nil

	s.Log("Waiting for falling back to AP0")
	if err := waitForRoam(); err != nil {
		s.Fatal("Failed to wait for falling back to AP0: ", err)
	}
	skipped, err = skippedRecver()
	if err != nil {
		s.Fatal("Failed to wait for confirming skipping EAP authentication: ", err)
	}
	if !skipped {
		s.Error("EAP authentication is not skipped")
	} else {
		s.Log("EAP authentication is skipped as expected")
	}

	if err := tf.VerifyConnection(ctx, ap0); err != nil {
		s.Error("Failed to verify connection to AP0 after fallbacking: ", err)
	}
}

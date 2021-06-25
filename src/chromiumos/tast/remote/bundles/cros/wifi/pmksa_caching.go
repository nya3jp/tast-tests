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
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PMKSACaching,
		Desc: "Verifies that 802.1x authentication (EAP exchange) is bypassed and PMKSA is done using PMK caching when it is available",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func PMKSACaching(ctx context.Context, s *testing.State) {
	const (
		ap0Channel  = 1
		ap1Channel  = 44
		roamTimeout = 30 * time.Second
	)
	// Generate BSSIDs for the two APs.
	mac0, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	mac1, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate BSSID: ", err)
	}
	ap0BSSID := mac0.String()
	ap1BSSID := mac1.String()

	tf := s.FixtValue().(*wificell.TestFixture)

	cert := certificate.TestCert1()
	secConfFac := wpaeap.NewConfigFactory(
		cert.CACred.Cert, cert.ServerCred,
		wpaeap.ClientCACert(cert.CACred.Cert),
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

	roamProps := func(bssid string) []*wificell.ShillProperty {
		return []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{bssid},
			Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
		}}
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
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap0)
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

	roamCtx, cancel := context.WithTimeout(ctx, roamTimeout)
	defer cancel()
	waitForRoam, err := tf.ExpectShillProperty(roamCtx, connResp.ServicePath, roamProps(ap1BSSID), nil)
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
	if _, err := waitForRoam(); err != nil {
		s.Fatal("Failed to wait for roaming to AP1: ", err)
	}
	skipped, err := skippedRecver()
	if err != nil {
		s.Fatal("Failed to wait for confirming skipping EAP authentication: ", err)
	}
	if skipped {
		s.Error("EAP authentication is skipped when connecting to a new AP")
	} else {
		s.Log("EAP authentication is not skipped as expected")
	}

	roamCtx, cancel = context.WithTimeout(ctx, roamTimeout)
	defer cancel()
	waitForRoam, err = tf.ExpectShillProperty(roamCtx, connResp.ServicePath, roamProps(ap0BSSID), nil)
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
	if _, err := waitForRoam(); err != nil {
		s.Fatal("Failed to wait for falling back to AP0: ", err)
	}
	skipped, err = skippedRecver()
	if err != nil {
		s.Fatal("Failed to wait for confirming skipping EAP authentication: ", err)
	}
	if !skipped {
		s.Error("EAP authentication is not skipped while expecting the PMKSA is cached")
	} else {
		s.Log("EAP authentication is skipped as expected")
	}

	// Shill may keep IsConnected during the whole roaming process if wpa_supplicant didn't explicitly tell that the DUT has disconnected.
	// However, we may still lose the connectivity for a while after roaming to the other BSSID. Use polling to verify the connection.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return tf.VerifyConnection(ctx, ap0)
	}, &testing.PollOptions{
		Timeout:  time.Second * 20,
		Interval: time.Second,
	}); err != nil {
		s.Error("Failed to wait for the connection to recover: ", err)
	}
}

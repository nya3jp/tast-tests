// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MBOAssocDisallow,
		Desc: "Verifies that a DUT won't connect to an AP with the assoc disallow bit set",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixt",
		SoftwareDeps: []string{"mbo"},
	})
}

func requestScanAndWaitForReport(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// Wait for the current scan (if any in progress) to finish just
	// to make sure that below we get scan report for a scan request
	// issued AFTER modification of MBOAssocDisallow
	if _, err := tf.WifiClient().WaitScanIdle(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to wait for scan to be idle")
	}
	wpaMonitor, stop, ctx, err := tf.StartWPAMonitor(ctx)
	if err != nil {
		s.Fatal("Failed to start wpa monitor")
	}
	defer stop()
	scanSuccess := false
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req := &wifi.RequestScansRequest{Count: 1}
	if _, err := tf.WifiClient().RequestScans(timeoutCtx, req); err != nil {
		s.Fatal("Failed to request scan: ", err)
	}
	for {
		event, err := wpaMonitor.WaitForEvent(timeoutCtx)
		if err != nil {
			s.Fatal("Failed to wait for scan event: ", err)
		}
		if event == nil { // timeout
			break
		}
		_, scanSuccess = event.(*wificell.ScanResultsEvent)
		if scanSuccess {
			break
		}
	}
	if !scanSuccess {
		s.Fatal("Unable to get scan results")
	}
}

func MBOAssocDisallow(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}

	s.Log("Configuring AP")
	testSSID := hostapd.RandomSSID("MBO_ASSOC_DISALLOW_")
	channel := 1
	apOpts := []hostapd.Option{hostapd.SSID(testSSID), hostapd.Mode(hostapd.Mode80211nMixed), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.Channel(channel), hostapd.MBO()}
	ap, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	freqOpts, err := ap.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get pcap frequency options: ", err)
	}

	s.Log("Setting assoc disallow")
	if err := ap.Set(ctx, hostapd.PropertyMBOAssocDisallow, "1"); err != nil {
		s.Fatal("Unable to set assoc disallow on AP: ", err)
	}

	// Make sure at least 1 scan with new setting is received
	requestScanAndWaitForReport(ctx, s)

	s.Log("Attempting to connect to AP")
	expectFailConnect := func(ctx context.Context) error {
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return nil
		}
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
		return errors.New("Unexpectedly connected to AP")
	}
	legacyRouter, err := tf.StandardRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}
	pcapPath, err := wifiutil.CollectPcapForAction(ctx, legacyRouter, "mbo_assoc_disallow", channel, freqOpts, expectFailConnect)
	if err != nil {
		s.Fatal("Failed to collect pcap: ", err)
	}

	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.Dot11FCSValid(),
		pcap.TransmitterAddress(mac),
		pcap.TypeFilter(layers.LayerTypeDot11MgmtAssociationReq, nil),
	}
	assocPackets, err := pcap.ReadPackets(pcapPath, filters...)
	if len(assocPackets) > 0 {
		s.Fatal("DUT sent assoc requests to the AP when it shouldn't have")
	}
	s.Log("Found no association requests as expected")
}

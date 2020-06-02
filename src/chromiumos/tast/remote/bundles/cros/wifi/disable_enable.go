// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"net"
	"strconv"

	"chromiumos/tast/common/network/iw"
	riw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        DisableEnable,
		Desc:        "Tests that disabling and enabling WiFi re-connects the system",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_cq", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func DisableEnable(fullCtx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(tfCtx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()
	ctx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap)
	defer cancel()
	s.Log("AP setup done")

	connRes, err := tf.ConnectWifi(ctx, ap.Config().Ssid, ap.Config().Hidden, ap.Config().SecurityConfig)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(fullCtx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ap.Config().Ssid, err)
		}
	}()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get interface from the DUT: ", err)
	}

	// Start disabling and enabling WiFi interface.
	if _, err := tf.WifiClient().DisableEnable(ctx, &network.DisableEnableRequest{
		InterfaceName: iface,
		ServicePath:   connRes.ServicePath,
	}); err != nil {
		s.Fatal("DisableEnable failed: ", err)
	}

	// Ensure that we have reconnected to the correct AP.

	// Check frequency.
	clientFreq, err := riw.NewRunner(s.DUT().Conn()).LinkValue(ctx, iface, iw.LinkKeyFrequency)
	if err != nil {
		s.Fatal("Failed to get client frequency: ", err)
	}
	serverFreq, err := hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		s.Fatal("Failed to get server frequency: ", err)
	}
	if clientFreq != strconv.Itoa(serverFreq) {
		s.Errorf("Frequency not match, want %d, got %s", serverFreq, clientFreq)
	}

	// Check subnet.
	addrs, err := tf.WifiClient().GetIPv4Addrs(ctx, &network.GetIPv4AddrsRequest{InterfaceName: iface})
	if err != nil {
		s.Fatal("Failed to get client ipv4 addresses: ", err)
	}
	serverSubnet := ap.ServerSubnet().String()
	foundSubnet := false
	for _, a := range addrs.Ipv4 {
		_, ipnet, err := net.ParseCIDR(a)
		if err != nil {
			s.Errorf("Failed to parse IP address %s: %v", a, err)
		}
		if ipnet.String() == serverSubnet {
			foundSubnet = true
			break
		}
	}
	if !foundSubnet {
		s.Errorf("Subnet not match, want %s, got addrs %v", serverSubnet, addrs.Ipv4)
	}

	// Check connectivity.
	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Error("Failed to ping from the DUT: ", err)
	}
}

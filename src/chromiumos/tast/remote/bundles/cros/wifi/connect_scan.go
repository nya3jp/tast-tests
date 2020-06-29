// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"reflect"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConnectScan,
		Desc:         "Verifies that the 802.11 probe frames with expected SSIDs are seen over-the-air when connecting to WiFi",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:         []string{"group:wificell", "wificell_func"},
		ServiceDeps:  []string{"tast.cros.network.WifiService"},
		Vars:         []string{"router", "pcap"},
		HardwareDeps: hwdep.D(hwdep.WifiMACAddrRandomize()),
		Params: []testing.Param{
			{
				Name:      "hidden",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []hostapd.Option{
					hostapd.Channel(48),
					hostapd.Mode(hostapd.Mode80211nPure),
					hostapd.HTCaps(hostapd.HTCapHT40),
					hostapd.Hidden(),
				},
			},
			{
				Name:      "visible",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []hostapd.Option{
					hostapd.Channel(1), // We have visible_vht for 5G band, use 2.4G band here.
					hostapd.Mode(hostapd.Mode80211nPure),
					hostapd.HTCaps(hostapd.HTCapHT40),
				},
			},
			{
				// For coverage of 5G and VHT setting.
				Name:      "visible_vht",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []hostapd.Option{
					hostapd.Channel(149),
					hostapd.Mode(hostapd.Mode80211acPure),
					hostapd.VHTChWidth(hostapd.VHTChWidth80),
					hostapd.VHTCenterChannel(155),
					hostapd.VHTCaps(hostapd.VHTCapSGI80),
					hostapd.HTCaps(hostapd.HTCapHT40Plus),
				},
			},
		},
	})
}

func ConnectScan(ctx context.Context, s *testing.State) {
	ops := []wificell.TFOption{
		wificell.TFCapture(true),
	}
	if router, _ := s.Var("router"); router != "" {
		ops = append(ops, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		ops = append(ops, wificell.TFPcap(pcap))
	}
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to tear down test fixture, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	// Disable MAC randomization as we're filtering the packets with MAC address.
	resp, err := tf.WifiClient().SetMACRandomize(ctx, &network.SetMACRandomizeRequest{Enable: false})
	if err != nil {
		s.Fatal("Failed to disable MAC randomization: ", err)
	}
	defer func(ctx context.Context) {
		if _, err := tf.WifiClient().SetMACRandomize(ctx, &network.SetMACRandomizeRequest{Enable: resp.OldSetting}); err != nil {
			s.Errorf("Failed to restore MAC randomization to %t: %v", resp.OldSetting, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

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

	apOps := s.Param().([]hostapd.Option)
	pcapPath, apConf, err := connectAndCollectPcap(ctx, tf, "pcap", apOps)
	if err != nil {
		s.Fatal("Failed to collect packet: ", err)
	}

	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				// Filter sender == MAC of DUT.
				return bytes.Equal(dot11.Address2, mac)
			},
		),
		pcap.TypeFilter(layers.LayerTypeDot11MgmtProbeReq, nil),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}
	s.Logf("Total %d probe requests found", len(packets))
	if len(packets) == 0 {
		s.Fatal("No probe request captured")
	}

	ssidSet := make(map[string]struct{})
	for _, p := range packets {
		layer := p.Layer(layers.LayerTypeDot11MgmtProbeReq)
		if layer == nil {
			s.Fatal("Found packet without PrboeReq layer")
		}
		ssid, err := pcap.ParseProbeReqSSID(layer.(*layers.Dot11MgmtProbeReq))
		if err != nil {
			// Let's be strict here as we've filtered source MAC and
			// packets with invalid FCS, so we don't expect malformed
			// packets here.
			s.Errorf("Malformed probe request %v: %v", layer, err)
			continue
		}
		ssidSet[ssid] = struct{}{}
	}

	expectedSSIDs := map[string]struct{}{
		"":          {},
		apConf.SSID: {},
	}
	if apConf.Hidden {
		// For hidden network, we expect both SSIDs.
		if !reflect.DeepEqual(ssidSet, expectedSSIDs) {
			s.Fatalf("Got set of SSIDs %v, want %v", ssidSet, expectedSSIDs)
		}
	} else {
		// For visible network, we expect wildcard SSID, but it is not guaranteed.
		for ssid := range ssidSet {
			if _, ok := expectedSSIDs[ssid]; !ok {
				s.Errorf("Found unexpected SSID=%q", ssid)
			}
		}
	}
}

// connectAndCollectPcap sets up a WiFi AP and then asks DUT to connect.
// The path to the packet file and the config of the AP is returned.
// Note: This function assumes that TestFixture spawns Capturer for us.
func connectAndCollectPcap(ctx context.Context, tf *wificell.TestFixture, name string, apOps []hostapd.Option) (pcapPath string, apConf *hostapd.Config, err error) {
	// As we'll collect pcap file after APIface and Capturer closed, run it
	// in an inner function so that we can clean up easier with defer.
	capturer, conf, err := func(ctx context.Context) (ret *pcap.Capturer, retConf *hostapd.Config, retErr error) {
		collectFirstErr := func(err error) {
			if retErr == nil {
				ret = nil
				retConf = nil
				retErr = err
			}
			testing.ContextLog(ctx, "Error in connectAndCollectPcap: ", err)
		}

		testing.ContextLog(ctx, "Configuring WiFi to connect")
		ap, err := tf.ConfigureAP(ctx, apOps, nil)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to configure AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to deconfig AP"))
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		testing.ContextLog(ctx, "Connecting to WiFi")
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return nil, nil, err
		}
		defer func(ctx context.Context) {
			if err := tf.DisconnectWifi(ctx); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to disconnect"))
			}
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				collectFirstErr(errors.Wrapf(err, "failed to remove entries for ssid=%s", ap.Config().SSID))
			}
		}(ctx)

		capturer, ok := tf.Capturer(ap)
		if !ok {
			return nil, nil, errors.New("cannot get the capturer from TestFixture")
		}
		return capturer, ap.Config(), nil
	}(ctx)
	if err != nil {
		return "", nil, err
	}
	pcapPath, err = capturer.PacketPath(ctx)
	if err != nil {
		return "", nil, err
	}
	return pcapPath, conf, nil
}

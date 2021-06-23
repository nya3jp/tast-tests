// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	remoteiw "chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OverlappingBSSScan,
		Desc: "Verifies that OBSS scan aborts and/or backs off when there is consistent outgoing traffic",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		Timeout:     5 * time.Minute,
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
	})
}

func OverlappingBSSScan(ctx context.Context, s *testing.State) {
	// To verify that OBSS scans will abort or back off when there's
	// outgoing traffic instead of blocking it, this test samples a long
	// period of pinging, and compares the maximum latency with or without
	// OBSS so that we can assume that our traffic does hit some running
	// scans if OBSS is enabled and it does not block the traffic too long
	// which then implies scan backs off.
	tf := s.FixtValue().(*wificell.TestFixture)

	// Turn off power save in this test as we are using ping RTT
	// as metric in this test. The default beacon interval (~100ms)
	// is too large compared with our threshold/margin and we'll
	// need much better resolution. Also, we don't want the timing
	// of beacons to interfere with our results.
	// e.g. default beacon interval is ~102ms and we might exceed
	// the 100ms threshold just because we send request right
	// after one beacon.
	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}
	psMode, err := iwr.PowersaveMode(ctx, clientIface)
	if err != nil {
		s.Fatal("Failed to get the powersave mode: ", err)
	}
	if psMode {
		defer func(ctx context.Context) {
			s.Logf("Restoring power save mode to %t", psMode)
			if err := iwr.SetPowersaveMode(ctx, clientIface, psMode); err != nil {
				s.Errorf("Failed to restore powersave mode to %t: %v", psMode, err)
			}
		}(ctx)
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, clientIface, false); err != nil {
			s.Fatal("Failed to turn off powersave: ", err)
		}
	}

	// AP options with(out) OBSS scan for this test.
	genAPOps := func(obss bool) []hostapd.Option {
		ops := []hostapd.Option{
			hostapd.Channel(6),
			hostapd.Mode(hostapd.Mode80211nPure),
			hostapd.HTCaps(hostapd.HTCapHT40),
		}
		if obss {
			ops = append(ops, hostapd.OBSSInterval(10))
		}
		return ops
	}

	// setupAndPing sets up an AP with(out) OBSS scan, connects DUT to it
	// and collects ping statistics. The Capturer object is also returned
	// so the caller can verify the OBSS scan setting works properly.
	setupAndPing := func(ctx context.Context, obss bool) (ret *ping.Result, retPcap *pcap.Capturer, retErr error) {
		const (
			pingInterval    = 0.1  // In seconds.
			pingCountOBSS   = 1000 // Total 100 seconds of ping-ing.
			pingCountNoOBSS = 100  // Total 10 seconds of ping-ing.
		)

		// Utility function for collecting errors in defer.
		collectErr := func(err error) {
			if err == nil {
				return
			}
			s.Log("Error in setupAndPing: ", err)
			if retErr == nil {
				ret = nil
				retPcap = nil
				retErr = err
			}
		}

		ap, err := tf.ConfigureAP(ctx, genAPOps(obss), nil)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to configure AP")
		}
		defer func(ctx context.Context) {
			s.Log("Deconfiguring AP")
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectErr(errors.Wrap(err, "failed to deconfig AP"))
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		s.Log("Connecting")
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to WiFi")
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				collectErr(errors.Wrap(err, "failed to disconnect WiFi"))
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		pr := remoteping.NewRemoteRunner(s.DUT().Conn())
		var count int
		var desc string
		var pingLogPath string
		if obss {
			desc = "with OBSS scan"
			count = pingCountOBSS
			pingLogPath = "ping_obss_enabled.log"
		} else {
			desc = "without OBSS scan"
			count = pingCountNoOBSS
			pingLogPath = "ping_obss_disabled.log"
		}
		s.Logf("Pinging router %s, count=%d, interval=%fs", desc, count, pingInterval)
		pingStats, err := pr.Ping(ctx, ap.ServerIP().String(), ping.Count(count),
			ping.Interval(pingInterval), ping.SaveOutput(pingLogPath))
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to ping router %s", desc)
		}
		s.Logf("Ping statistic %s: %v", desc, pingStats)

		capturer, ok := tf.Capturer(ap)
		if !ok {
			return nil, nil, errors.New("no capturer spawned")
		}

		return pingStats, capturer, nil
	}

	// The latency thresholds in ms to match the unit of ping.Result.
	const (
		latencyBaseline = 100
		// Dwell time for scanning is usually configured to be around 100 ms (some
		// are higher, around 150 ms), since this is also the standard beacon
		// interval. Tolerate spikes in latency up to 250 ms as a way of asking that
		// our PHY be servicing foreground traffic regularly during background scans.
		latencyMargin = 250
	)

	statsNoBgscan, _, err := setupAndPing(ctx, false)
	if err != nil {
		s.Fatal("Failed to measure latency without OBSS scan: ", err)
	}
	if statsNoBgscan.MaxLatency > latencyBaseline {
		s.Fatalf("RTT latency is too high even without OBSS scan: %f ms > %f ms",
			statsNoBgscan.MaxLatency, float64(latencyBaseline))
	}

	statsBgscan, capturer, err := setupAndPing(ctx, true)
	if err != nil {
		s.Fatal("Failed to measure latency with OBSS scan: ", err)
	}
	if statsBgscan.MaxLatency > statsNoBgscan.MaxLatency+latencyMargin {
		s.Errorf("Significant difference in RTT due to OBSS scan: diff RTT (%f ms with OBSS - %f ms without OBSS) > %f ms",
			statsBgscan.MaxLatency, statsNoBgscan.MaxLatency, float64(latencyMargin))
	}

	s.Log("Parsing packets to see if coexistence management frames are sent")
	// Get the MAC address of DUT's WiFi interface.
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, clientIface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface")
	}

	pcapPath, err := capturer.PacketPath(ctx)
	if err != nil {
		s.Fatal("Failed to get path of packet file: ", err)
	}

	// Filtering coexistence management frame.
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
		pcap.TypeFilter(layers.LayerTypeDot11MgmtAction,
			func(layer gopacket.Layer) bool {
				contents := layer.LayerContents()
				// Check fixed parameter:
				//   contents[0]: Category = Public Action (4)
				//   contents[1]: Action = 20/40 BSS Coexistence Management (0)
				if len(contents) < 2 {
					return false
				}
				if contents[0] != 4 || contents[1] != 0 {
					return false
				}
				// Parse tagged parameters to find 20/40 BSS coexistence element.
				e := gopacket.NewPacket(contents[2:], layers.LayerTypeDot11InformationElement, gopacket.NoCopy)
				if err := e.ErrorLayer(); err != nil {
					// Malformed packet, log and skip.
					s.Logf("Found malformed coexistence management frame, content=%v, err=%v", contents, err)
					return false
				}
				for _, l := range e.Layers() {
					element, ok := l.(*layers.Dot11InformationElement)
					if !ok {
						// Unexpected layer, log and skip the packet.
						s.Log("Found unexpected layer when parsing informantion element ", l)
						return false
					}
					if element.ID == layers.Dot11InformationElementID2040BSSCoExist {
						return true
					}
				}
				return false
			},
		),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}
	s.Logf("Total %d packets found", len(packets))
	if len(packets) == 0 {
		s.Fatal("No coexistence management packet found in pcap")
	}
}

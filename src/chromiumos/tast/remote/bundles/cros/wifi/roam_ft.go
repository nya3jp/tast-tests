// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type roamFTparam struct {
	secConfFac security.ConfigFactory
	mixed      bool
}

var (
	roamFTCert1 = certificate.TestCert1()
	roamFTCert2 = certificate.TestCert2()
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        RoamFT,
		Desc:        "Verifies that DUT can roam with FT auth suites",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{{
			Name:      "psk",
			ExtraAttr: []string{"wificell_unstable"},
			Val: roamFTparam{
				secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP), wpa.FTMode(wpa.FTModePure)),
			},
		}, {
			Name:      "mixed_psk",
			ExtraAttr: []string{"wificell_unstable"},
			Val: roamFTparam{
				secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP), wpa.FTMode(wpa.FTModeMixed)),
				mixed:      true,
			},
		}, {
			Name:      "eap",
			ExtraAttr: []string{"wificell_unstable"},
			Val: roamFTparam{
				secConfFac: wpaeap.NewConfigFactory(
					roamFTCert1.CACert, roamFTCert1.ServerCred,
					wpaeap.ClientCACert(roamFTCert1.CACert), wpaeap.ClientCred(roamFTCert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2), wpaeap.FTMode(wpa.FTModePure),
				),
			},
		}, {
			Name:      "mixed_eap",
			ExtraAttr: []string{"wificell_unstable"},
			Val: roamFTparam{
				secConfFac: wpaeap.NewConfigFactory(
					roamFTCert1.CACert, roamFTCert1.ServerCred,
					wpaeap.ClientCACert(roamFTCert1.CACert), wpaeap.ClientCred(roamFTCert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2), wpaeap.FTMode(wpa.FTModeMixed),
				),
				mixed: true,
			},
		}},
	})
}

func RoamFT(ctx context.Context, s *testing.State) {
	/*
		Roaming using FT is different from standard roaming in that there
		is a special key exchange protocol that needs to occur between the
		APs prior to a successful roam. In order for this communication to
		work, we need to construct a specific interface architecture as
		shown below:
		             _________                       _________
		            |         |                     |         |
		            |   br0   |                     |   br1   |
		            |_________|                     |_________|
		           ____|   |____                   ____|   |____
		     _____|____     ____|____         ____|____     ____|_____
		    |          |   |         |       |         |   |          |
		    | managed0 |   |  veth0  | <---> |  veth1  |   | managed1 |
		    |__________|   |_________|       |_________|   |__________|

		The managed0 and managed1 interfaces cannot communicate with each
		other without a bridge. However, the same bridge cannot be used
		to bridge the two interfaces either (as soon as managed0 is bound
		to a bridge, hostapd would notice and would configure the same MAC
		address as managed0 onto the bridge, and send/recv the L2 packet
		with the bridge). Thus, we create a virtual ethernet interface with
		one peer on either bridge to allow the bridges to forward traffic
		between managed0 and managed1.
	*/
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	// Shorten a second for releasing each network device.
	reserveForRelease := func(ctx context.Context) (context.Context, func()) {
		return ctxutil.Shorten(ctx, time.Second)
	}

	apID := 0
	uniqueAPName := func() string {
		id := strconv.Itoa(apID)
		apID++
		return id
	}

	// runOnce sets up the network environment as mentioned above, and verifies the DUT is able to roam between the APs iff expectedFailure is not set.
	runOnce := func(ctx context.Context, secConfFac security.ConfigFactory, expectedFailure bool) {
		var err error
		var br [2]string
		for i := 0; i < 2; i++ {
			br[i], err = tf.Router().NewBridge(ctx)
			if err != nil {
				s.Fatal("Failed to get a bridge: ", err)
			}
			defer func(ctx context.Context, b string) {
				if err := tf.Router().ReleaseBridge(ctx, b); err != nil {
					s.Error("Failed to release bridge: ", err)
				}
			}(ctx, br[i])
			ctx, cancel = reserveForRelease(ctx)
			defer cancel()
		}

		var veth [2]string
		veth[0], veth[1], err = tf.Router().NewVethPair(ctx)
		if err != nil {
			s.Fatal("Failed to get a veth pair: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.Router().ReleaseVethPair(ctx, veth[0]); err != nil {
				s.Error("Failed to release veth: ", err)
			}
		}(ctx)
		ctx, cancel = reserveForRelease(ctx)
		defer cancel()

		// Bind the two ends of the veth to the two bridges.
		for i := 0; i < 2; i++ {
			if err := tf.Router().BindVethToBridge(ctx, veth[i], br[i]); err != nil {
				s.Fatalf("Failed to bind the veth %q to bridge %q: %v", veth[i], br[i], err)
			}
			defer func(ctx context.Context, ve string) {
				if err := tf.Router().UnbindVeth(ctx, ve); err != nil {
					s.Errorf("Failed to unbind %q: %v", ve, err)
				}
			}(ctx, veth[i])
			ctx, cancel = reserveForRelease(ctx)
			defer cancel()
		}

		s.Logf("Network environment setup is done: %s <= %s----%s => %s", br[0], veth[0], veth[1], br[1])

		mac0, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to get a random mac address: ", err)
		}
		mac1, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to get a random mac address: ", err)
		}
		var (
			id0 = hex.EncodeToString(mac0)
			id1 = hex.EncodeToString(mac1)
		)
		mac := []net.HardwareAddr{mac0, mac1}
		const (
			key0 = "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100"
			key1 = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
			mdID = "a1b2"
		)

		ap0SecConf, err := secConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		ap0Ops := []hostapd.Option{
			hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g), hostapd.BSSID(mac0.String()),
			hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id0), hostapd.R1KeyHolder(id0),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac1, id1, key0)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac1, mac1, key1)),
			hostapd.Bridge(br[0]), hostapd.SecurityConfig(ap0SecConf),
		}
		ap0Conf, err := hostapd.NewConfig(ap0Ops...)
		if err != nil {
			s.Fatal("Failed to generate the hostapd config for AP0: ", err)
		}
		ap1SecConf, err := secConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		ap1Ops := []hostapd.Option{
			hostapd.SSID(ap0Conf.SSID), hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.BSSID(mac1.String()),
			hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155),
			hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id1), hostapd.R1KeyHolder(id1),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac0, id0, key1)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac0, mac0, key0)),
			hostapd.Bridge(br[1]), hostapd.SecurityConfig(ap1SecConf),
		}
		ap1Conf, err := hostapd.NewConfig(ap1Ops...)
		if err != nil {
			s.Fatal("Failed to generate the hostapd config for AP1: ", err)
		}

		s.Log("Starting the first AP on ", br[0])
		ap0Name := uniqueAPName()
		var ap []*hostapd.Server
		ap0, err := tf.Router().StartHostapd(ctx, ap0Name, ap0Conf)
		if err != nil {
			s.Fatal("Failed to start the hostapd server on the first AP: ", err)
		}
		ap = append(ap, ap0)
		defer func(ctx context.Context) {
			if err := tf.Router().StopHostapd(ctx, ap[0]); err != nil {
				s.Error("Failed to stop the hostapd server on the first AP: ", err)
			}
		}(ctx)
		ctx, cancel = ap[0].ReserveForClose(ctx)
		defer cancel()

		var (
			serverIP    = net.IPv4(192, 168, 0, 254)
			startIP     = net.IPv4(192, 168, 0, 1)
			endIP       = net.IPv4(192, 168, 0, 128)
			broadcastIP = net.IPv4(192, 168, 0, 255)
			mask        = net.IPv4Mask(255, 255, 255, 0)
		)
		s.Logf("Starting the DHCP server on %s, serverIP=%s", br[0], serverIP)
		ds, err := tf.Router().StartDHCP(ctx, ap0Name, br[0], startIP, endIP, serverIP, broadcastIP, mask)
		if err != nil {
			s.Fatal("Failed to start the DHCP server: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.Router().StopDHCP(ctx, ds); err != nil {
				s.Error("Failed to stop the DHCP server: ", err)
			}
		}(ctx)
		ctx, cancel = ds.ReserveForClose(ctx)
		defer cancel()
		pingF := func() (wificell.ResultType, error) {
			pr := remoteping.NewRemoteRunner(s.DUT().Conn())
			//ap0.ServerIP().String()
			res, err := pr.Ping(ctx, serverIP.String(), ping.Count(100))
			if err != nil {
				testing.ContextLog(ctx, "ping error, ", err)
				return wificell.ResultType{}, err
			}
			testing.ContextLogf(ctx, "ping statistics=%+v", res)

			return wificell.ResultType{Data: res.Loss, Timestamp: time.Now()}, nil
		}
		vf := wificell.NewVerifier(ctx, pingF)

		connResp, err := tf.ConnectWifi(ctx, ap[0].Config().SSID, wificell.ConnSecurity(ap0SecConf))
		if err != nil {
			if expectedFailure {
				s.Log("Failed to connect to the AP as expected; Tearing down")
				return
			}
			s.Fatal("Failed to connect to the AP: ", err)
		}
		if expectedFailure {
			s.Fatal("Expected failure but succeeded")
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect from the AP: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		if err := tf.PingFromDUT(ctx, serverIP.String()); err != nil {
			s.Fatal("Failed to ping from the DUT: ", err)
		}

		s.Log("Connected to the first AP; Start roaming")
		// TODO(b/171086223): The changing of BSSID only means we've initiated the roam attempt, as opposed to be actually L3 connected. Remove the polling below once the bug is addressed.
		props := [][]*wificell.ShillProperty{{{
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{mac1.String()},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		}}, {{
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{mac0.String()},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		}}}

		vf.StartJob()

		s.Log("Starting the second AP on ", br[1])
		ap1Name := uniqueAPName()
		ap1, err := tf.Router().StartHostapd(ctx, ap1Name, ap1Conf)
		if err != nil {
			s.Fatal("Failed to start the hostapd server on the second AP: ", err)
		}
		ap = append(ap, ap1)
		defer func(ctx context.Context) {
			if err := tf.Router().StopHostapd(ctx, ap[1]); err != nil {
				s.Error("Failed to stop the hostapd server on the second AP: ", err)
			}
		}(ctx)
		ctx, cancel = ap[1].ReserveForClose(ctx)
		defer cancel()

		iface, err := tf.ClientInterface(ctx)
		if err != nil {
			s.Fatal("Failed to get interface from the DUT: ", err)
		}

		for i := 0; i < 10; i++ {
			waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			waitForProps, err := tf.ExpectShillProperty(waitCtx, connResp.ServicePath, props[i%2], []string{shillconst.ServicePropertyIsConnected})
			if err != nil {
				s.Fatal("Failed to create a property watcher: ", err)
			}

			discCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if err := tf.DiscoverBSSID(discCtx, mac[(i+1)%2].String(), iface, []byte(ap[(i+1)%2].Config().SSID)); err != nil {
				s.Fatalf("Failed to discover the BSSID %s: %v", mac[(i+1)%2].String(), err)
			}

			s.Logf("Step %d. Requesting roam from %s to %s", i, mac[i%2], mac[(i+1)%2])
			// Request shill to send D-Bus roam request to wpa_supplicant.
			if err := tf.RequestRoam(ctx, iface, mac[(i+1)%2].String(), 15*time.Second); err != nil {
				s.Fatalf("Failed to roam from %s to %s: %v", mac[i%2], mac[(i+1)%2], err)
			}

			monitorResult, err := waitForProps()
			if err != nil {
				s.Fatal("Failed to wait for the properties: ", err)
			}
			// Check that we don't disconnect along the way here, in case we're ping-ponging around APs --
			// and after the first (failed) roam, the second re-connection will not be testing FT at all.
			for _, ph := range monitorResult {
				if ph.Name == shillconst.ServicePropertyIsConnected {
					if !ph.Value.(bool) {
						s.Error("Failed to stay connected during the roaming process")
					}
				}
			}

			// Verify the L3 connectivity and make sure that the DUT stays connected to the second AP.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := tf.PingFromDUT(ctx, serverIP.String()); err != nil {
					return err
				}
				dutState, err := tf.QueryService(ctx)
				if err != nil {
					return err
				}
				if dutState.Wifi.Bssid != mac[(i+1)%2].String() {
					return testing.PollBreak(errors.Errorf("unexpected BSSID: got %s, want %s", dutState.Wifi.Bssid, mac[(i+1)%2]))
				}
				return nil
			}, &testing.PollOptions{
				Timeout:  30 * time.Second,
				Interval: time.Second,
			}); err != nil {
				s.Error("Failed to verify the connection: ", err)
			}
		}
		verificatonResults(ctx, vf)
	}
	hasFTSupport := func(ctx context.Context) bool {
		phys, err := iw.NewRemoteRunner(s.DUT().Conn()).ListPhys(ctx)
		if err != nil {
			s.Fatal("Failed to check SME capability: ", err)
		}
		for _, p := range phys {
			for _, c := range p.Commands {
				// A DUT which has SME capability should support FT.
				if c == "authenticate" {
					return true
				}
				// A full-mac driver that supports update_ft_ies functions also supports FT.
				if c == "update_ft_ies" {
					return true
				}
			}
		}
		return false
	}

	ctx, restoreBg, err := tf.TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off the background scan: ", err)
	}
	defer func() {
		if err := restoreBg(); err != nil {
			s.Error("Failed to restore the background scan config: ", err)
		}
	}()

	ftResp, err := tf.WifiClient().GetGlobalFTProperty(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get the global FT property: ", err)
	}
	defer func(ctx context.Context) {
		if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &network.SetGlobalFTPropertyRequest{Enabled: ftResp.Enabled}); err != nil {
			s.Errorf("Failed to set global FT property back to %v: %v", ftResp.Enabled, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	param := s.Param().(roamFTparam)
	// Turn on the global FT and test once.
	if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &network.SetGlobalFTPropertyRequest{Enabled: true}); err != nil {
		s.Fatal("Failed to turn on the global FT property: ", err)
	}
	// Expect failure if we are running pure FT test and the DUT is not supporting SME.
	runOnce(ctx, param.secConfFac, !param.mixed && !hasFTSupport(ctx))
	// Run the test without global FT. It should pass iff we configured the AP in mixed mode.
	if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &network.SetGlobalFTPropertyRequest{Enabled: false}); err != nil {
		s.Fatal("Failed to turn off the global FT property: ", err)
	}
	runOnce(ctx, param.secConfFac, !param.mixed)
}

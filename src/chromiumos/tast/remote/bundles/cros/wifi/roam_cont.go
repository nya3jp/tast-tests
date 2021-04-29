// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/verifier"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type contParam struct {
	secConfFac security.ConfigFactory
	enableFT   bool
	rounds     int
	maxLoss    float32
}

var (
	contCert1 = certificate.TestCert1()
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RoamCont,
		Desc: "Verifies that DUT can roam with FT auth suites",
		Contacts: []string{
			"jck@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap", "wifi.RoamCont.rounds"},
		Params: []testing.Param{{
			Name:      "none",
			ExtraAttr: []string{"wificell_unstable"},
			Val: contParam{
				rounds:  50,
				maxLoss: 0.05,
			},
		}, {
			Name:      "psk",
			ExtraAttr: []string{"wificell_unstable"},
			Val: contParam{
				rounds:     50,
				maxLoss:    0.05,
				secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
			},
		}, {
			Name:      "ft_psk",
			ExtraAttr: []string{"wificell_unstable"},
			Val: contParam{
				rounds:     50,
				maxLoss:    0.05,
				secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP), wpa.FTMode(wpa.FTModeMixed)),
				enableFT:   true,
			},
		}, {
			Name:      "eap",
			ExtraAttr: []string{"wificell_unstable"},
			Val: contParam{
				rounds:  50,
				maxLoss: 0.05,
				secConfFac: wpaeap.NewConfigFactory(
					contCert1.CACred.Cert, contCert1.ServerCred,
					wpaeap.ClientCACert(contCert1.CACred.Cert), wpaeap.ClientCred(contCert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
			},
		}, {
			Name:      "ft_eap",
			ExtraAttr: []string{"wificell_unstable"},
			Val: contParam{
				rounds:  50,
				maxLoss: 0.05,
				secConfFac: wpaeap.NewConfigFactory(
					contCert1.CACred.Cert, contCert1.ServerCred,
					wpaeap.ClientCACert(contCert1.CACred.Cert), wpaeap.ClientCred(contCert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2), wpaeap.FTMode(wpa.FTModeMixed),
				),
				enableFT: true,
			},
		}},
	})
}

func RoamCont(ctx context.Context, s *testing.State) {
	/*
		Because we need single DHCP server sharing same address pool for
		clients connected to either AP, we use a setup similar to RoamFT:
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

	param := s.Param().(contParam)
	// Turn on the global FT.
	if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &network.SetGlobalFTPropertyRequest{Enabled: param.enableFT}); err != nil {
		s.Fatal("Failed to set the global FT property: ", err)
	}

	// Allow override
	roundsStr, _ := s.Var("wifi.RoamCont.rounds")
	if roundsStr == "" {
		roundsStr = strconv.Itoa(param.rounds)
	}
	rounds, err := strconv.Atoi(roundsStr)
	if err != nil {
		s.Fatal("Failed to convert value, err: ", err)
	}

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
	var mac [2]net.HardwareAddr
	mac[0], err = hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to get a random mac address: ", err)
	}
	mac[1], err = hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to get a random mac address: ", err)
	}
	var (
		id0 = hex.EncodeToString(mac[0])
		id1 = hex.EncodeToString(mac[1])
	)
	const (
		key0 = "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100"
		key1 = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
		mdID = "a1b2"
	)

	ssid := hostapd.RandomSSID("TAST_ROAM_CONT_")

	// Basic config
	ap0Ops := []hostapd.Option{
		hostapd.SSID(ssid), hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g), hostapd.BSSID(mac[0].String()),
		hostapd.Bridge(br[0]),
	}
	ap1Ops := []hostapd.Option{
		hostapd.SSID(ssid), hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.BSSID(mac[1].String()),
		hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155),
		hostapd.Bridge(br[1]),
	}

	// FT Enabled
	if param.enableFT {
		ap0Ops = append(ap0Ops, hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id0), hostapd.R1KeyHolder(id0),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac[1], id1, key0)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac[1], mac[1], key1)))
		ap1Ops = append(ap1Ops, hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id1), hostapd.R1KeyHolder(id1),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac[0], id0, key1)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac[0], mac[0], key0)))
	}

	// Security enabled
	if param.secConfFac != nil {
		ap0SecConf, err := param.secConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		ap0Ops = append(ap0Ops, hostapd.SecurityConfig(ap0SecConf))
		ap1SecConf, err := param.secConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		ap1Ops = append(ap1Ops, hostapd.SecurityConfig(ap1SecConf))
	}

	ap0Conf, err := hostapd.NewConfig(ap0Ops...)
	if err != nil {
		s.Fatal("Failed to generate the hostapd config for AP0: ", err)
	}
	ap1Conf, err := hostapd.NewConfig(ap1Ops...)
	if err != nil {
		s.Fatal("Failed to generate the hostapd config for AP1: ", err)
	}

	s.Log("Starting the first AP on ", br[0])
	ap0Name := uniqueAPName()
	var ap [2]*hostapd.Server
	ap[0], err = tf.Router().StartHostapd(ctx, ap0Name, ap0Conf)
	if err != nil {
		s.Fatal("Failed to start the hostapd server on the first AP: ", err)
	}
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

	pingF := func() (verifier.ResultType, error) {
		pr := remoteping.NewRemoteRunner(s.DUT().Conn())
		res, err := pr.Ping(ctx, serverIP.String(), ping.Count(1000), ping.Interval(0.01))
		if err != nil {
			testing.ContextLog(ctx, "ping error: ", err)
			return verifier.ResultType{}, err
		}
		testing.ContextLogf(ctx, "Continuity: ping statistics=%+v", res)
		return verifier.ResultType{Data: res, Timestamp: time.Now()}, nil
	}
	vf := verifier.NewVerifier(ctx, pingF)
	defer vf.Finish()
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	connResp, err := tf.ConnectWifi(ctx, ap[0].Config().SSID, wificell.ConnSecurity(ap[0].Config().SecurityConfig))
	if err != nil {
		s.Fatal("Failed to connect to the AP: ", err)
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
	props := [][]*wificell.ShillProperty{{{
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateConfiguration},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateReady},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateIdle},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{mac[1].String()},
		Method:         network.ExpectShillPropertyRequest_CHECK_ONLY,
	}}, {{
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateConfiguration},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateReady},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateIdle},
		Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{mac[0].String()},
		Method:         network.ExpectShillPropertyRequest_CHECK_ONLY,
	}}}

	s.Log("Starting the second AP on ", br[1])
	ap1Name := uniqueAPName()
	ap[1], err = tf.Router().StartHostapd(ctx, ap1Name, ap1Conf)
	if err != nil {
		s.Fatal("Failed to start the hostapd server on the second AP: ", err)
	}
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
	vf.StartJob()
	for i := 0; i < rounds; i++ {
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

		dutState, err := tf.QueryService(ctx)
		if err != nil {
			s.Fatal("Failed to query service: ", err)
		}
		if dutState.Wifi.Bssid != mac[(i+1)%2].String() {
			s.Fatalf("Unexpected BSSID: got %s, want %s", dutState.Wifi.Bssid, mac[(i+1)%2])
		}
	}
	results, err := vf.StopJob()
	if err != nil {
		testing.ContextLog(ctx, "Error while receiving verification results: ", err)
		return
	}
	var sent, received int
	for i, ret := range results {
		pingData := ret.Data.(*ping.Result)
		testing.ContextLogf(ctx, "Iteration %d: Time=%s, Loss rate=%+v",
			i+1, ret.Timestamp.Format("15:04:05.000"), pingData.Loss)
		sent += pingData.Sent
		received += pingData.Received
	}
	loss := sent - received
	testing.ContextLogf(ctx, "Total loss rate=%+v%% (%d/%d)",
		(loss * 100 / sent), loss, sent)

	if 1.0*loss/sent > int(param.maxLoss) {
		s.Fatal("Loss threshold exceeded")
	}
}

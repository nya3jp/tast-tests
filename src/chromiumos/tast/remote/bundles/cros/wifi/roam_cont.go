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
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/verifier"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type contTestType int

const (
	pingTest contTestType = iota
)

type contParam struct {
	apOpts     [2][]hostapd.Option
	secConfFac security.ConfigFactory
	enableFT   bool
	rounds     int
	testType   contTestType
	param      interface{}
}

type pingParam struct {
	opts []ping.Option
	// Max packets lost per roaming round.
	maxLoss int
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
		Attr:        []string{"group:wificell", "wificell_perf", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap", "wifi.RoamCont.rounds"},
		Params: []testing.Param{{
			Name: "none",
			Val: contParam{
				testType: pingTest,
				rounds:   50,
				apOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				param: pingParam{
					opts: []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					// With 10ms ping interval, abnormal time without service would be 25 packets = 250ms.
					maxLoss: 25,
				},
			},
		}, {
			Name: "psk",
			Val: contParam{
				testType: pingTest,
				rounds:   50,
				apOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
		}, {
			Name: "ft_psk",
			Val: contParam{
				testType: pingTest,
				rounds:   50,
				apOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP), wpa.FTMode(wpa.FTModePure)),
				enableFT:   true,
				param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
		}, {
			Name: "eap",
			Val: contParam{
				testType: pingTest,
				rounds:   50,
				apOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				secConfFac: wpaeap.NewConfigFactory(
					contCert1.CACred.Cert, contCert1.ServerCred,
					wpaeap.ClientCACert(contCert1.CACred.Cert), wpaeap.ClientCred(contCert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
				param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
		}, {
			Name: "ft_eap",
			Val: contParam{
				testType: pingTest,
				rounds:   50,
				apOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				secConfFac: wpaeap.NewConfigFactory(
					contCert1.CACred.Cert, contCert1.ServerCred,
					wpaeap.ClientCACert(contCert1.CACred.Cert), wpaeap.ClientCred(contCert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2), wpaeap.FTMode(wpa.FTModePure),
				),
				enableFT: true,
				param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
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

	if param.enableFT && !hasFTSupport(ctx) {
		s.Fatal("Unable to run FT test on device not supporting FT")
	}

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
	var apOps [2][]hostapd.Option

	// Basic config
	for i := 0; i < 2; i++ {
		apOps[i] = append(apOps[i], hostapd.SSID(ssid), hostapd.BSSID(mac[i].String()), hostapd.Bridge(br[i]))
		apOps[i] = append(apOps[i], param.apOpts[i]...)
	}

	// FT Enabled
	if param.enableFT {
		apOps[0] = append(apOps[0], hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id0), hostapd.R1KeyHolder(id0),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac[1], id1, key0)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac[1], mac[1], key1)))
		apOps[1] = append(apOps[1], hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id1), hostapd.R1KeyHolder(id1),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac[0], id0, key1)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac[0], mac[0], key0)))
	}

	// Security enabled
	if param.secConfFac != nil {
		ap0SecConf, err := param.secConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		apOps[0] = append(apOps[0], hostapd.SecurityConfig(ap0SecConf))
		ap1SecConf, err := param.secConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		apOps[1] = append(apOps[1], hostapd.SecurityConfig(ap1SecConf))
	}

	ap0Conf, err := hostapd.NewConfig(apOps[0]...)
	if err != nil {
		s.Fatal("Failed to generate the hostapd config for AP0: ", err)
	}
	ap1Conf, err := hostapd.NewConfig(apOps[1]...)
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

	if err := tf.PingFromDUT(ctx, serverIP.String(), ping.Count(3), ping.Interval(0.3)); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	var vf *verifier.Verifier
	var resultAssertF func(context.Context, []verifier.ResultType)
	switch param.testType {
	case pingTest:
		pingF := func(ctx context.Context) (verifier.ResultType, error) {
			// We need more result data than a simple tf.PingFromDUT(), so we use a separate runner.
			pr := remoteping.NewRemoteRunner(s.DUT().Conn())
			res, err := pr.Ping(ctx, serverIP.String(), param.param.(pingParam).opts...)
			if err != nil {
				testing.ContextLog(ctx, "ping error: ", err)
				return verifier.ResultType{}, err
			}
			testing.ContextLogf(ctx, "Continuity: ping statistics=%+v", res)
			return verifier.ResultType{Data: res, Timestamp: time.Now()}, nil
		}
		resultAssertF = func(ctx context.Context, results []verifier.ResultType) {
			var sent, received int
			for i, ret := range results {
				pingData := ret.Data.(*ping.Result)
				testing.ContextLogf(ctx, "Iteration %d: End Time=%s, Packets lost=%d",
					i+1, ret.Timestamp.Format("15:04:05.000"), pingData.Sent-pingData.Received)
				sent += pingData.Sent
				received += pingData.Received
			}
			loss := sent - received
			testing.ContextLogf(ctx, "Total packets lost=%d/%d (%d per round)",
				loss, sent, loss/rounds)

			if loss > rounds*param.param.(pingParam).maxLoss {
				s.Fatal("Loss threshold exceeded")
			}
		}
		vf = verifier.NewVerifier(ctx, pingF)
	default:
		s.Fatal("Unknown test type")
	}
	defer vf.Finish()
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

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
	var lastScan time.Time

	// Wrap the test round in a separate function so all defers fire properly.
	testRound := func(ctx context.Context, s *testing.State, round int) {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		waitForProps, err := tf.ExpectShillProperty(waitCtx, connResp.ServicePath, props[round%2], []string{shillconst.ServicePropertyIsConnected})
		if err != nil {
			s.Fatal("Failed to create a property watcher: ", err)
		}
		if time.Since(lastScan) > 30*time.Second {
			lastScan = time.Now()
			discCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if err := tf.DiscoverBSSID(discCtx, mac[(round+1)%2].String(), iface, []byte(ap[(round+1)%2].Config().SSID)); err != nil {
				s.Fatalf("Failed to discover the BSSID %s: %v", mac[(round+1)%2].String(), err)
			}
		}
		s.Logf("Round %d. Requesting roam from %s to %s", round+1, mac[round%2], mac[(round+1)%2])

		// Request shill to send D-Bus roam request to wpa_supplicant.
		if err := tf.RequestRoam(ctx, iface, mac[(round+1)%2].String(), 15*time.Second); err != nil {
			s.Fatalf("Failed to roam from %s to %s: %v", mac[round%2], mac[(round+1)%2], err)
		}

		monitorResult, err := waitForProps()
		if err != nil {
			s.Fatal("Failed to wait for the properties: ", err)
		}

		// Check that we don't disconnect along the way here, in case we're ping-ponging around APs.
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
		if dutState.Wifi.Bssid != mac[(round+1)%2].String() {
			s.Fatalf("Unexpected BSSID: got %s, want %s", dutState.Wifi.Bssid, mac[(round+1)%2])
		}
	}
	for i := 0; i < rounds; i++ {
		testRound(ctx, s, i)
	}
	results, err := vf.StopJob()
	if err != nil {
		testing.ContextLog(ctx, "Error while receiving verification results: ", err)
		return
	}
	resultAssertF(ctx, results)
}

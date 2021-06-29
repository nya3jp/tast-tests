// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// ContParam holds all parameters for the continuity test.
type ContParam struct {
	ApOpts     [2][]hostapd.Option
	SecConfFac security.ConfigFactory
	EnableFT   bool
	Rounds     int
	Param      interface{}
}

// ContTest hods all varibles to be accessible for the whole continuity test.
type ContTest struct {
	tf          *wificell.TestFixture
	r           router.Legacy
	clientMAC   string
	br          [2]string
	veth        [2]string
	mac         [2]net.HardwareAddr
	apOps       [2][]hostapd.Option
	ap          [2]*hostapd.Server
	dserv       *dhcp.Server
	ftEnabled   bool
	iface       string
	restoreBg   func() error
	servicePath string
}

var apID int

var (
	serverIP    = net.IPv4(192, 168, 0, 254)
	startIP     = net.IPv4(192, 168, 0, 1)
	endIP       = net.IPv4(192, 168, 0, 128)
	broadcastIP = net.IPv4(192, 168, 0, 255)
	mask        = net.IPv4Mask(255, 255, 255, 0)
)

// Cert1 defines a certificate used for testing.
var Cert1 = certificate.TestCert1()

func reserveForRelease(ctx context.Context) (context.Context, func()) {
	return ctxutil.Shorten(ctx, 10*time.Second)
}

func uniqueAPName() string {
	id := strconv.Itoa(apID)
	apID++
	return id
}

func hasFTSupport(ctx context.Context, s *testing.State) bool {
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

// ServerIP returns server IP used for the test.
func ServerIP() string {
	return serverIP.String()
}

// ContinuityTestInitialSetup performs the initial setup of the test environment.
func ContinuityTestInitialSetup(ctx context.Context, s *testing.State, tf *wificell.TestFixture) (context.Context, *ContTest, func()) {
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
	ct := &ContTest{tf: tf}
	ds, destroyIfNotExported := newDestructorStack()
	defer destroyIfNotExported()

	var err error
	ctx, ct.restoreBg, err = ct.tf.TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off the background scan: ", err)
	}
	ds.push(func() {
		if err := ct.restoreBg(); err != nil {
			s.Error("Failed to restore the background scan config: ", err)
		}
	})

	ct.clientMAC, err = tf.ClientHardwareAddr(ctx)
	if err != nil {
		s.Fatal("Unable to get DUT MAC address: ", err)
	}

	ftResp, err := ct.tf.WifiClient().GetGlobalFTProperty(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get the global FT property: ", err)
	}
	ct.ftEnabled = ftResp.Enabled

	param := s.Param().(ContParam)

	// TODO(b/190630644): Add HWDeps to cover such devices.
	if param.EnableFT && !hasFTSupport(ctx, s) {
		s.Fatal("Unable to run FT test on device not supporting FT")
	}

	// Turn on the global FT.
	if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &wifi.SetGlobalFTPropertyRequest{Enabled: param.EnableFT}); err != nil {
		s.Fatal("Failed to set the global FT property: ", err)
	}
	ds.push(func() {
		if _, err := ct.tf.WifiClient().SetGlobalFTProperty(ctx, &wifi.SetGlobalFTPropertyRequest{Enabled: ct.ftEnabled}); err != nil {
			s.Errorf("Failed to set global FT property back to %v: %v", ct.ftEnabled, err)
		}
	})

	ct.r, err = tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	ct.veth[0], ct.veth[1], err = ct.r.NewVethPair(ctx)
	if err != nil {
		s.Fatal("Failed to get a veth pair: ", err)
	}
	ds.push(func() {
		if err := ct.r.ReleaseVethPair(ctx, ct.veth[0]); err != nil {
			s.Error("Failed to release veth: ", err)
		}
	})
	// Bind the two ends of the veth to the two bridges.
	for i := 0; i < 2; i++ {
		ct.br[i], err = ct.r.NewBridge(ctx)
		if err != nil {
			s.Fatal("Failed to get a bridge: ", err)
		}
		if err := ct.r.BindVethToBridge(ctx, ct.veth[i], ct.br[i]); err != nil {
			s.Fatalf("Failed to bind the veth %q to bridge %q: %v", ct.veth[i], ct.br[i], err)
		}
		ct.mac[i], err = hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to get a random mac address: ", err)
		}
	}
	ds.push(func() {
		if err := ct.r.ReleaseBridge(ctx, ct.br[0]); err != nil {
			s.Error("Failed to release bridge: ", err)
		}
		if err := ct.r.ReleaseBridge(ctx, ct.br[1]); err != nil {
			s.Error("Failed to release bridge: ", err)
		}
	})

	s.Logf("Network environment setup is done: %s <= %s----%s => %s", ct.br[0], ct.veth[0], ct.veth[1], ct.br[1])
	var (
		id0 = hex.EncodeToString(ct.mac[0])
		id1 = hex.EncodeToString(ct.mac[1])
	)
	const (
		key0 = "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100"
		key1 = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
		mdID = "a1b2"
	)

	ssid := hostapd.RandomSSID("TAST_ROAM_CONT_")

	// Basic config
	for i := 0; i < 2; i++ {
		ct.apOps[i] = append(ct.apOps[i], hostapd.SSID(ssid), hostapd.BSSID(ct.mac[i].String()), hostapd.Bridge(ct.br[i]))
		ct.apOps[i] = append(ct.apOps[i], param.ApOpts[i]...)
	}

	// FT Enabled
	if param.EnableFT {
		ct.apOps[0] = append(ct.apOps[0], hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id0), hostapd.R1KeyHolder(id0),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", ct.mac[1], id1, key0)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", ct.mac[1], ct.mac[1], key1)))
		ct.apOps[1] = append(ct.apOps[1], hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id1), hostapd.R1KeyHolder(id1),
			hostapd.R0KHs(fmt.Sprintf("%s %s %s", ct.mac[0], id0, key1)),
			hostapd.R1KHs(fmt.Sprintf("%s %s %s", ct.mac[0], ct.mac[0], key0)))
	}

	// Security enabled
	if param.SecConfFac != nil {
		ap0SecConf, err := param.SecConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		ct.apOps[0] = append(ct.apOps[0], hostapd.SecurityConfig(ap0SecConf))
		ap1SecConf, err := param.SecConfFac.Gen()
		if err != nil {
			s.Fatal("Failed to generate security config: ", err)
		}
		ct.apOps[1] = append(ct.apOps[1], hostapd.SecurityConfig(ap1SecConf))
	}

	ap0Conf, err := hostapd.NewConfig(ct.apOps[0]...)
	if err != nil {
		s.Fatal("Failed to generate the hostapd config for AP0: ", err)
	}
	ds.push(func() {
		if err := ct.r.StopHostapd(ctx, ct.ap[0]); err != nil {
			s.Error("Failed to stop the hostapd server on the first AP: ", err)
		}
	})
	s.Log("Starting the first AP on ", ct.br[0])
	ap0Name := uniqueAPName()
	ct.ap[0], err = ct.r.StartHostapd(ctx, ap0Name, ap0Conf)
	if err != nil {
		s.Fatal("Failed to start the hostapd server on the first AP: ", err)
	}

	s.Logf("Starting the DHCP server on %s, serverIP=%s", ct.br[0], serverIP)
	ct.dserv, err = ct.r.StartDHCP(ctx, ap0Name, ct.br[0], startIP, endIP, serverIP, broadcastIP, mask)
	if err != nil {
		s.Fatal("Failed to start the DHCP server: ", err)
	}
	ds.push(func() {
		if err := ct.r.StopDHCP(ctx, ct.dserv); err != nil {
			s.Error("Failed to stop the DHCP server: ", err)
		}
	})

	connResp, err := tf.ConnectWifi(ctx, ct.ap[0].Config().SSID, dutcfg.ConnSecurity(ct.ap[0].Config().SecurityConfig))
	if err != nil {
		s.Fatal("Failed to connect to the AP: ", err)
	}
	ds.push(func() {
		if err := ct.tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect from the AP: ", err)
		}
	})
	ct.servicePath = connResp.ServicePath

	if err := tf.PingFromDUT(ctx, serverIP.String(), ping.Count(3), ping.Interval(0.3)); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}
	s.Log("Connected to the first AP")

	return ctx, ct, ds.export().destroy
}

// ContinuityTestSetupFinalize finalizes the setup of the test environment.
func (ct *ContTest) ContinuityTestSetupFinalize(ctx context.Context, s *testing.State) (context.Context, func()) {
	s.Log("Starting the second AP")
	ds, destroyIfNotExported := newDestructorStack()
	defer destroyIfNotExported()
	ap1Name := uniqueAPName()
	ap1Conf, err := hostapd.NewConfig(ct.apOps[1]...)
	if err != nil {
		s.Fatal("Failed to generate the hostapd config for AP1: ", err)
	}

	ct.ap[1], err = ct.r.StartHostapd(ctx, ap1Name, ap1Conf)
	if err != nil {
		s.Fatal("Failed to start the hostapd server on the second AP: ", err)
	}
	ds.push(func() {
		if err := ct.r.StopHostapd(ctx, ct.ap[1]); err != nil {
			s.Error("Failed to stop the hostapd server on the second AP: ", err)
		}
	})

	ct.iface, err = ct.tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get interface from the DUT: ", err)
	}

	return ctx, ds.export().destroy
}

// ContinuityRound runs one round of the test: scans if necessary and attempts roaming.
func (ct *ContTest) ContinuityRound(ctx context.Context, s *testing.State, round int) {
	var props [][]*wificell.ShillProperty = [][]*wificell.ShillProperty{{{
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateConfiguration},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateReady},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateIdle},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{ct.mac[1].String()},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
	}}, {{
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateConfiguration},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateReady},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiRoamState,
		ExpectedValues: []interface{}{shillconst.RoamStateIdle},
		Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
	}, {
		Property:       shillconst.ServicePropertyWiFiBSSID,
		ExpectedValues: []interface{}{ct.mac[0].String()},
		Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
	}}}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	waitForProps, err := ct.tf.ExpectShillProperty(waitCtx, ct.servicePath, props[round%2], []string{shillconst.ServicePropertyIsConnected})
	if err != nil {
		s.Fatal("Failed to create a property watcher: ", err)
	}
	s.Logf("Round %d. Requesting roam from %s to %s", round+1, ct.mac[round%2], ct.mac[(round+1)%2])

	s.Logf("Sending BSS TM Request from AP %s to DUT %s", ct.mac[round%2], ct.clientMAC)
	req := hostapd.BSSTMReqParams{Neighbors: []string{ct.mac[(round+1)%2].String()}}
	if err := ct.ap[round%2].SendBSSTMRequest(ctx, ct.clientMAC, req); err != nil {
		s.Fatal("Failed to send BSS TM Request: ", err)
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

	dutState, err := ct.tf.QueryService(ctx)
	if err != nil {
		s.Fatal("Failed to query service: ", err)
	}
	if dutState.Wifi.Bssid != ct.mac[(round+1)%2].String() {
		s.Fatalf("Unexpected BSSID: got %s, want %s", dutState.Wifi.Bssid, ct.mac[(round+1)%2])
	}
}

// Router returns current router object.
func (ct *ContTest) Router() router.Legacy {
	return ct.r
}

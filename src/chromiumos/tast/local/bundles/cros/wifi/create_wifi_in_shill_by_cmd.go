// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// wifiInShillByCmdTestCase describes the parameters of a single test case.
type wifiInShillByCmdTestCase struct {
	// cmd is the list of args consisting of a cmd that will be run to create network.
	cmd []string
	// SSID of the network to be added.
	ssid string
	// props are expected properties of created network
	props map[string]string
}

func init() {
	// Set of tests designed to reproduce command line interaction to create network
	// from ARC. It is expected for shill to successfully create WiFi network with
	// given arguments.
	testing.AddTest(&testing.Test{
		Func: CreateWifiInShillByCmd,
		Desc: "Test if wifi network can be correctly created in shill",
		Contacts: []string{
			"chuweih@google.com",
			"cros-networking@google.com",
		},
		Fixture:      "shillSimulatedWiFiWithArcBooted",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi", "chrome", "arc"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "auto_reconnect_disabled",
				Val: wifiInShillByCmdTestCase{
					cmd:  []string{"-d"},
					ssid: "GoogleGuest",
					props: map[string]string{
						"AutoConnect": "false",
					},
				},
			},
			{
				Name: "dns_and_search_domains",
				Val: wifiInShillByCmdTestCase{
					cmd:  []string{"--dns", "8.8.8.8,8.8.4.4", "--search-domains", "test1.com,test2.com"},
					ssid: "GoogleGuest",
					props: map[string]string{
						"AutoConnect":   "true",
						"NameServers":   "8.8.8.8,8.8.4.4",
						"SearchDomains": "test1.com,test2.com",
					},
				},
			},
			{
				Name: "auto_reconnect_disabled_with_pac_url",
				Val: wifiInShillByCmdTestCase{
					cmd:  []string{"-d", "--pac", "http://test/test"},
					ssid: "GoogleGuest",
					props: map[string]string{
						"AutoConnect":              "false",
						"WebProxyAutoDiscoveryUrl": "http://test/test",
					},
				},
			},
			{
				Name: "manual_proxy",
				Val: wifiInShillByCmdTestCase{
					cmd:  []string{"-d", "--proxy-host", "hostName", "--proxy-port", "2222"},
					ssid: "GoogleGuest",
					props: map[string]string{
						"AutoConnect": "false",
						"ManualProxy": "true",
					},
				},
			},
		},
	})
}

// CreateWifiInShillByCmd expects a wifi with given configs are created correctly through shell cmd.
func CreateWifiInShillByCmd(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to shill Manager: ", err)
	}

	a := s.FixtValue().(*hwsim.ShillSimulatedWiFi).ARC
	tc := s.Param().(wifiInShillByCmdTestCase)
	// Reserve a little time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	// Append customized args of every test case to base cmd that is shared to form a cmd.
	baseCmd := []string{"wifi", "add-network", "GoogleGuest", "open"}
	out, err := a.Command(ctx, "cmd", append(baseCmd, tc.cmd...)...).Output(testexec.DumpLogOnError)
	// Forget newly added network after each test.
	defer forgetNetwork(ctx, a, tc.ssid)

	// If the output contains failure message, it indicates failure to add network.
	if err != nil || regexp.MustCompile(`.*Save failed.*`).Match(out) {
		s.Error("Failed to add network: ", err)
	}

	services, _, err := m.ServicesByTechnology(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Error("Failed to get wifi services: ", err)
	}

	for _, service := range services {
		p, err := service.GetProperties(ctx)
		if err != nil {
			s.Error("Failed to get wifi service properties: ", err)
		}

		// Get the SSID of current wifi service.
		hexSSID, err := p.GetString(shillconst.ServicePropertyWiFiHexSSID)
		decodeSSID, err := hex.DecodeString(hexSSID)
		if err != nil {
			s.Error("Failed to decode SSID: ", err)
		}

		curSSID := string(decodeSSID)

		// If SSID of current service does not match SSID in the test case, means that it is
		// not newly added wifi network, continue.
		if curSSID != tc.ssid {
			continue
		}

		// Check if properties of newly added network match expectations.
		for prop := range tc.props {
			switch prop {
			case "AutoConnect":
				b, err := p.GetBool(prop)
				if err != nil {
					s.Error("Failed to get auto connect property from service: ", err)
				}

				if strconv.FormatBool(b) != tc.props[prop] {
					s.Errorf("autoreconnect = %t, want: %t : %v", strconv.FormatBool(b), tc.props[prop], err)
				}
			case "WebProxyAutoDiscoveryUrl":
				proxy, err := p.Get("ProxyConfig")
				if err != nil {
					s.Error("Failed to get proxy config property from service: ", err)
				}

				expected := tc.props[prop]
				configs := strings.Split(proxy.(string), ",")
				if !strings.Contains(proxy.(string), "pac_url") {
					s.Error("No pac url is set: ", err)
				}

				for _, config := range configs {
					if strings.Contains(config, "pac_url") {
						if !strings.Contains(config, expected) {
							s.Errorf("pac url = %v, want: %v : %v", config, tc.props[prop], err)
						}
					}
				}
			case "ManualProxy":
				proxy, err := p.Get("ProxyConfig")
				if err != nil {
					s.Error("Failed to get proxy config property from service: ", err)
				}

				configs := proxy.(string)
				if !strings.Contains(configs, "pac_script") {
					s.Error("do not have manual config set: ", err)
				}
			case "NameServers":
				staticIPConfig, err := p.Get(shillconst.ServicePropertyStaticIPConfig)
				if err != nil {
					s.Error("Failed to get static IP config property from service: ", err)
				}

				nameServers := staticIPConfig.(map[string]interface{})[shillconst.IPConfigPropertyNameServers].([]string)
				expected := strings.Split(tc.props[prop], ",")
				for _, ns := range expected {
					if !containsElement(nameServers, ns) {
						s.Errorf(ns+" is expected to be included but it is not: %v", err)
					}
				}
			case "SearchDomains":
				staticIPConfig, err := p.Get(shillconst.ServicePropertyStaticIPConfig)
				if err != nil {
					s.Error("Failed to get static IP config property from service: ", err)
				}

				searchDomains := staticIPConfig.(map[string]interface{})["SearchDomains"]
				expected := strings.Split(tc.props[prop], ",")
				if !reflect.DeepEqual(expected, searchDomains.([]string)) {
					s.Errorf("searchDomains is: %+v, however %+v is expected: %v", searchDomains.([]string), expected, err)
				}
			}
		}
	}
}

func containsElement(list []string, a string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func forgetNetwork(ctx context.Context, a *arc.ARC, ssid string) error {
	out, err := a.Command(ctx, "cmd", "wifi", "list-networks").Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	for _, network := range strings.Split(string(out), "\n") {
		// Get network information by shell cmd, if the information line is empty, skip.
		if len(network) == 0 {
			continue
		}
		netInfos := strings.Split(network, " ")

		// The output is in the format of:
		// networkid=<netID> SSID=<SSID> BSSID=<BSSID> guid=<guid> security=<security>
		// currentSSID is the SSID of current network. If currentSSID matches newly added
		// network, get networkId of this network and forget network through shell cmd.
		currentSSID := strings.Split(string(netInfos[1]), "\"")[1]

		if currentSSID == ssid {
			netID := strings.Split(netInfos[0], "=")[1]
			if err := a.Command(ctx, "cmd", "wifi", "forget-network", netID).Run(testexec.DumpLogOnError); err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

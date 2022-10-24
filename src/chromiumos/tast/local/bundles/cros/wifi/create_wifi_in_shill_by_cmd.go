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
	"chromiumos/tast/errors"
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
					cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "-d"},
					ssid: "GoogleGuest",
					props: map[string]string{
						"AutoConnect": "false",
					},
				},
			},
			{
				Name: "dns_and_search_domains",
				Val: wifiInShillByCmdTestCase{
					cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "--dns", "8.8.8.8,8.8.4.4", "--search-domains", "test1.com,test2.com"},
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
					cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "-d", "--pac", "http://test/test"},
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
					cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "-d", "--proxy-host", "hostName", "--proxy-port", "2222"},
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
	if err := runAddNetworkTestCases(ctx, s, m, a, tc); err != nil {
		s.Errorf("Failed to complete creating wifi test with %s: %v", tc, err)
	}
}

func runAddNetworkTestCases(ctx context.Context, s *testing.State, m *shill.Manager, a *arc.ARC, tc wifiInShillByCmdTestCase) (retErr error) {
	// Reserve a little time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	out, err := a.Command(ctx, "cmd", tc.cmd...).Output(testexec.DumpLogOnError)
	// Forget newly added network after each test.
	defer forgetNetwork(ctx, a, tc.ssid)

	// If the output contains failure message, it indicates failure to add network.
	if err != nil || regexp.MustCompile(`.*Save failed.*`).Match(out) {
		return errors.Wrap(err, "failed to add network")
	}

	services, _, err := m.ServicesByTechnology(ctx, shill.TechnologyWifi)
	if err != nil {
		return errors.Wrap(err, "failed to get wifi services")
	}

	for _, service := range services {
		p, err := service.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get wifi service properties")
		}

		// Get the SSID of current wifi service.
		hexSSID, err := p.GetString(shillconst.ServicePropertyWiFiHexSSID)
		decodeSSID, err := hex.DecodeString(hexSSID)
		if err != nil {
			return errors.Wrap(err, "failed to decode SSID")
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
					return errors.Wrap(err, "failed to get auto connect property from service")
				}

				if strconv.FormatBool(b) != tc.props[prop] {
					return errors.Wrap(err, "autoreconnect = "+strconv.FormatBool(b)+", want: "+tc.props[prop])
				}
			case "WebProxyAutoDiscoveryUrl":
				proxy, err := p.Get("ProxyConfig")
				if err != nil {
					return errors.Wrap(err, "failed to get proxy config property from service")
				}

				expected := tc.props[prop]
				configs := strings.Split(proxy.(string), ",")
				if !strings.Contains(proxy.(string), "pac_url") {
					return errors.Wrap(err, "no pac url is set")
				}

				for _, config := range configs {
					if strings.Contains(config, "pac_url") {
						if !strings.Contains(config, expected) {
							return errors.Wrap(err, "pac url = "+config+", want: "+tc.props[prop])
						}
					}
				}
			case "ManualProxy":
				proxy, err := p.Get("ProxyConfig")
				if err != nil {
					return errors.Wrap(err, "failed to get proxy config property from service")
				}

				configs := proxy.(string)
				if !strings.Contains(configs, "pac_script") {
					return errors.Wrap(err, "do not have manual config set")
				}
			case "NameServers":
				staticIPConfig, err := p.Get(shillconst.ServicePropertyStaticIPConfig)
				if err != nil {
					return errors.Wrap(err, "failed to get static IP config property from service")
				}

				nameServers := staticIPConfig.(map[string]interface{})[shillconst.IPConfigPropertyNameServers].([]string)
				expected := strings.Split(tc.props[prop], ",")
				for _, ns := range expected {
					if !containsElement(nameServers, ns) {
						return errors.Wrap(err, ns+" is expected to be included but it is not")
					}
				}
			case "SearchDomains":
				staticIPConfig, err := p.Get(shillconst.ServicePropertyStaticIPConfig)
				if err != nil {
					return errors.Wrap(err, "failed to get static IP config property from service")
				}

				searchDomains := staticIPConfig.(map[string]interface{})["SearchDomains"]
				expected := strings.Split(tc.props[prop], ",")
				if !reflect.DeepEqual(expected, searchDomains.([]string)) {
					return errors.Wrapf(err, "searchDomains is: %+v, however %+v is expected", searchDomains.([]string), expected)
				}
			}
		}
	}
	return nil
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
		netInfos := strings.Split(network, " ")
		if len(network) == 0 {
			continue
		}

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

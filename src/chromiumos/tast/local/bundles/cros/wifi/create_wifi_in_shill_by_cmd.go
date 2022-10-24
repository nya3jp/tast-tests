// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"reflect"
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

func init() {
	// Set of tests designed to reproduce command line interaction to create network
	// from ARC. It is expected for shill to successfully create wifi network with
	// Given arguments.
	testing.AddTest(&testing.Test{
		Func: CreateWifiInShillByCmd,
		Desc: "Test if wifi network can be correctly created in shil",
		Contacts: []string{
			"chuweih@google.com",
			"cros-networking@google.com",
		},
		Fixture:      "shillSimulatedWiFiWithArcBooted",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi", "chrome", "arc"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      7 * time.Minute,
	})
}

type createWifiInShillByCmdTestCase struct {
	cmd   []string
	ssid  string
	props map[string]string
}

func CreateWifiInShillByCmd(ctx context.Context, s *testing.State) {
	var tcs = []createWifiInShillByCmdTestCase{
		{
			cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "-d"},
			ssid: "GoogleGuest",
			props: map[string]string{
				"AutoConnect": "false",
			},
		},
		{
			cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "--dns", "1.1.1.1,2.2.2.2", "--search-domains", "test1.com,test2.com"},
			ssid: "GoogleGuest",
			props: map[string]string{
				"AutoConnect":   "true",
				"NameServers":   "1.1.1.1,2.2.2.2",
				"SearchDomains": "test1.com,test2.com",
			},
		},
		{
			cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "-d", "--pac", "http://test/test"},
			ssid: "GoogleGuest",
			props: map[string]string{
				"AutoConnect":              "false",
				"WebProxyAutoDiscoveryUrl": "http://test/test",
			},
		},
		{
			cmd:  []string{"wifi", "add-network", "GoogleGuest", "open", "--proxy-host", "hostName", "--proxy-port", "2222"},
			ssid: "GoogleGuest",
			props: map[string]string{
				"AutoConnect": "true",
				"ManualProxy": "true",
			},
		},
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to shill Manager: ", err)
	}

	a := s.FixtValue().(*hwsim.ShillSimulatedWiFi).ARC
	for _, tc := range tcs {
		if err := runAddNetworkTestCases(ctx, s, m, a, tc); err != nil {
			s.Errorf("Failed to complete creating wifi test with %s: %v", tc, err)
		}
	}
}

func runAddNetworkTestCases(ctx context.Context, s *testing.State, m *shill.Manager, a *arc.ARC, tc createWifiInShillByCmdTestCase) (retErr error) {
	// Reserve a little time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	// Forget newly added network after each test.
	out, err := a.Command(ctx, "cmd", tc.cmd...).Output(testexec.DumpLogOnError)
	defer func() {
		out, err := a.Command(ctx, "cmd", "wifi", "list-networks").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}

		networks := strings.Split(string(out), "\n")
		for _, network := range networks {
			netInfos := strings.Split(network, " ")
			if len(network) == 0 {
				continue
			}

			currentSsid := strings.Split(string(netInfos[1]), "\"")[1]
			// If SSID matches newly added network, get networkId and forget network.
			if currentSsid == ssid {
				netID := strings.Split(netInfos[0], "=")[1]
				if err := a.Command(ctx, "cmd", "wifi", "forget-network", netID).Run(testexec.DumpLogOnError); err != nil {
					return err
				}
				return nil
			}
		}
		return nil
	}()

	if err != nil || string(out) == "Save failed\n" {
		return errors.Wrap(err, "failed to add network")
	}

	services, _, err := m.ServicesByTechnology(ctx, shill.TechnologyWifi)
	if err != nil {
		return err
	}

	for _, service := range services {
		p, err := service.GetProperties(ctx)
		if err != nil {
			return err
		}

		hexSsid, err := p.GetString(shillconst.ServicePropertyWiFiHexSSID)
		decodedSsid, _ := hex.DecodeString(hexSsid)
		curSsid := string(decodedSsid)

		// If current survice if not newly added wifi network, then continue
		if curSsid != tc.ssid {
			continue
		}

		for prop := range tc.props {
			switch prop {
			case "AutoConnect":
				b, _ := p.GetBool(prop)
				if strconv.FormatBool(b) != tc.props[prop] {
					return errors.Wrap(err, "expecting: "+tc.props[prop]+" but got: "+strconv.FormatBool(b))
				}
			case "WebProxyAutoDiscoveryUrl":
				proxy, _ := p.Get("ProxyConfig")
				expected := tc.props[prop]
				configs := strings.Split(proxy.(string), ",")
				if !strings.Contains(proxy.(string), "pac_url") {
					return errors.Wrap(err, "no pac url is set")
				}

				for _, config := range configs {
					if strings.Contains(config, "pac_url") {
						if !strings.Contains(config, expected) {
							return errors.Wrap(err, "expecting: "+tc.props[prop]+" but got: "+config)
						}
					}
				}
			case "ManualProxy":
				proxy, _ := p.Get("ProxyConfig")
				configs := proxy.(string)
				if !strings.Contains(configs, "pac_script") {
					return errors.Wrap(err, "do not have manual config set")
				}
			case "NameServers":
				staticIPConfig, _ := p.Get(shillconst.ServicePropertyStaticIPConfig)
				nameServers := staticIPConfig.(map[string]interface{})[shillconst.IPConfigPropertyNameServers].([]string)
				expected := strings.Split(tc.props[prop], ",")
				for _, ns := range expected {
					if !containsElement(nameServers, ns) {
						return errors.Wrap(err, ns+" is expected to be included but it is not")
					}
				}
			case "SearchDomains":
				staticIPConfig, _ := p.Get(shillconst.ServicePropertyStaticIPConfig)
				searchDomains := staticIPConfig.(map[string]interface{})["SearchDomains"]
				expected := strings.Split(tc.props[prop], ",")
				if !reflect.DeepEqual(expected, searchDomains.([]string)) {
					return errors.Wrapf(err, "searchDomains is expected to be: %+v, however it is: %+v", expected, searchDomains.([]string))
				}
			default:
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

// func ForgetNetwork(ctx context.Context, a *arc.ARC, ssid string) err {

// 	return nil
// }

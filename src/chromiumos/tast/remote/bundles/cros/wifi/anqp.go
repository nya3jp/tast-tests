// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"net"
	"reflect"
	"time"

	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type anqpTestCase struct {
	infos              hostapd.VenueInfo
	names              []string
	roamingConsortiums []string
	domains            []string
	realms             []hostapd.NAIRealm
}

var (
	methodEapTLS = hostapd.EAPMethod{
		Type: hostapd.EAPMethodTypeTLS,
		Params: []hostapd.EAPAuthParam{
			{
				Type:  hostapd.AuthParamCredential,
				Value: hostapd.AuthCredentialsCertificate,
			},
		},
	}
	methodEapTTLS = hostapd.EAPMethod{
		Type: hostapd.EAPMethodTypeTTLS,
		Params: []hostapd.EAPAuthParam{
			{Type: hostapd.AuthParamInnerNonEAP, Value: hostapd.AuthNonEAPAuthMSCHAPV2},
			{Type: hostapd.AuthParamCredential, Value: hostapd.AuthCredentialsUsernamePassword},
		},
	}
	tlsAndTtlsRealm = hostapd.NAIRealm{
		Domains:  []string{"example.com"},
		Encoding: hostapd.RealmEncodingRFC4282,
		Methods: []hostapd.EAPMethod{
			methodEapTLS,
			methodEapTTLS,
		},
	}
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ANQP,
		Desc: "Verifies that a DUT is able to perform ANQP requests and process replies",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Timeout:     10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "anqp_basic_info",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoBar,
					names:              []string{"eng:Foo bar"},
					roamingConsortiums: []string{"ab56cd8971"},
					domains:            []string{"example.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_names",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoZooOrAquarium,
					names:              []string{"eng:Local zoo park", "fra:Parc zoologique"},
					roamingConsortiums: []string{"9836fc"},
					domains:            []string{"example.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_ois",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoHotelOrMotel,
					names:              []string{"eng:My favorite hotel"},
					roamingConsortiums: []string{"001bc50050", "001bc500b5"},
					domains:            []string{"example.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_domains",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoGasStation,
					names:              []string{"eng:Foo station"},
					roamingConsortiums: []string{"2233445566"},
					domains:            []string{"blue.net", "red.com", "green.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_realms",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoAmphitheater,
					names:              []string{"eng:Bar amphitheater"},
					roamingConsortiums: []string{"0105b5c5"},
					domains:            []string{"foo.net", "bar.com", "example.net"},
					realms: []hostapd.NAIRealm{
						{
							Domains:  []string{"foo.net"},
							Encoding: hostapd.RealmEncodingRFC4282,
							Methods:  []hostapd.EAPMethod{methodEapTLS},
						}, {
							Domains:  []string{"bar.com", "example.net"},
							Encoding: hostapd.RealmEncodingUTF8,
							Methods:  []hostapd.EAPMethod{methodEapTTLS},
						},
					},
				},
			},
		},
	})
}

type anqpTestContext struct {
	ops                []hostapd.Option
	bssid              net.HardwareAddr
	venueInfo          hostapd.VenueInfo
	roamingConsortiums map[string]bool
	venueNames         map[string]bool
	domains            map[string]bool
	realms             []hostapd.NAIRealm
}

func ANQP(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	params := s.Param().(anqpTestCase)

	tc, err := prepareTestContext(params)
	if err != nil {
		s.Fatal("Failed to generate hostapd configuration: ", err)
	}

	ap, err := tf.ConfigureAP(ctx, tc.ops, nil)
	if err != nil {
		s.Fatal("Failed to configure access point: ", err)
	}
	defer tf.DeconfigAP(ctx, ap)

	monitor, monitorStop, sCtx, err := tf.StartWPAMonitor(ctx)
	if err != nil {
		s.Fatal("Failed to start WPA monitor: ", err)
	}
	defer monitorStop()

	runner := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: s.DUT().Conn()})

	// Trigger a scan and wait for the results to be available.
	if err := scanAndWait(sCtx, runner, monitor); err != nil {
		s.Fatal("Scan failed: ", err)
	}

	// Ask wpa_supplicant to fetch ANQP data for all the compatible access point
	// found in range during last scan.
	if err := fetchANQPAndWait(sCtx, runner, monitor, tc.bssid); err != nil {
		s.Fatal("ANQP fetch failed: ", err)
	}

	// Gather the data for the test access point.
	bss, err := runner.BSS(sCtx, tc.bssid)
	if err != nil {
		s.Fatal("Failed to obtain BSS: ", err)
	}

	// Check venue information match the test case.
	if len(tc.venueNames) > 0 {
		info, names, err := wifiutil.DecodeVenueGroupTypeName(bss["anqp_venue_name"])
		if err != nil {
			s.Fatal("Failed to parse venue name: ", err)
		}

		if !reflect.DeepEqual(info, tc.venueInfo) {
			s.Fatalf("Venue information does not match: got %v want %v", info, tc.venueInfo)
		}

		if len(names) != len(tc.venueNames) {
			s.Fatalf("Venue names count does not match: got %d want %d", len(names), len(tc.venueNames))
		}
		for _, name := range names {
			if ok := tc.venueNames[name]; !ok {
				s.Fatalf("Venue name %q provided by the access point is not expected", name)
			}
		}
	}

	// Check roaming consortium OIs are consistent with the test case.
	if len(tc.roamingConsortiums) > 0 {
		rcs, err := wifiutil.DecodeRoamingConsortiums(bss["anqp_roaming_consortium"])
		if err != nil {
			s.Fatal("Failed to parse roaming consortiums: ", err)
		}

		if len(rcs) != len(tc.roamingConsortiums) {
			s.Fatalf("Roaming consortiums count does not match: got %d want %d", len(rcs), len(tc.roamingConsortiums))
		}
		for _, rc := range rcs {
			if ok := tc.roamingConsortiums[rc]; !ok {
				s.Fatalf("Romaing consortium OI %q provided by the access point is not expected", rc)
			}
		}
	}

	// Check domain names are gathered from the AP.
	if len(tc.domains) > 0 {
		domains, err := wifiutil.DecodeDomainNames(bss["anqp_domain_name"])
		if err != nil {
			s.Fatal("Failed to parse domain names: ", err)
		}
		if len(domains) != len(tc.domains) {
			s.Fatalf("Domain names count does not match: got %d want %d", len(domains), len(tc.domains))
		}
		for _, d := range domains {
			if ok := tc.domains[d]; !ok {
				s.Fatalf("Domain name %q provided by the access point is not expected", d)
			}
		}
	}

	// Check the realms are identical to the ones configured.
	if len(tc.realms) > 0 {
		realms, err := wifiutil.DecodeNAIRealms(bss["anqp_nai_realm"])
		if err != nil {
			s.Fatal("Failed to parse NAI Realms: ", err)
		}
		if !reflect.DeepEqual(realms, tc.realms) {
			s.Fatalf("Realms are not matching: got %v want %v", realms, tc.realms)
		}
	}
}

func prepareTestContext(t anqpTestCase) (*anqpTestContext, error) {
	ssid := hostapd.RandomSSID("TAST_ANQP_")

	randAddr, err := hostapd.RandomMAC()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate BSSID")
	}

	ops := []hostapd.Option{
		hostapd.SSID(ssid),
		hostapd.Channel(1),
		hostapd.BSSID(randAddr.String()),
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.HTCaps(hostapd.HTCapHT20),
		hostapd.Interworking(),
		hostapd.VenueInfos(t.infos),
		hostapd.VenueNames(t.names...),
		hostapd.RoamingConsortiums(t.roamingConsortiums...),
		hostapd.DomainNames(t.domains...),
		hostapd.Realms(t.realms...),
	}

	names := make(map[string]bool)
	for _, name := range t.names {
		names[name] = true
	}

	ois := make(map[string]bool)
	for _, oi := range t.roamingConsortiums {
		ois[oi] = true
	}

	domains := make(map[string]bool)
	for _, domain := range t.domains {
		domains[domain] = true
	}

	return &anqpTestContext{
		ops:                ops,
		bssid:              randAddr,
		venueInfo:          t.infos,
		venueNames:         names,
		roamingConsortiums: ois,
		domains:            domains,
		realms:             t.realms,
	}, nil
}

// scanAndWait triggers a scan in wpa_supplicant and wait through the monitor
// for scan results to be available.
func scanAndWait(ctx context.Context, r *wpacli.Runner, m *wificell.WPAMonitor) error {
	if err := r.Scan(ctx); err != nil {
		return errors.Wrap(err, "failed to scan")
	}
	for {
		event, err := m.WaitForEvent(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to wait for scan results")
		}
		if _, ok := event.(*wificell.ScanResultsEvent); ok {
			break
		}
	}
	return nil
}

// fetchANQPAndWait triggers ANQP fetch sequence for all the compatible access
// points in range and wait for the queries to be done for the specified BSSID.
func fetchANQPAndWait(ctx context.Context, r *wpacli.Runner, m *wificell.WPAMonitor, bssid net.HardwareAddr) error {
	if err := r.FetchANQP(ctx); err != nil {
		return errors.Wrap(err, "failed to fetch ANQP")
	}
	for {
		event, err := m.WaitForEvent(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to wait for ANQP query done event")
		}
		if e, ok := event.(*wificell.ANQPQueryDoneEvent); ok {
			if bytes.Compare(e.Addr, bssid) != 0 {
				// Only consider ANQP requests results for the AP used in the test.
				continue
			}
			if e.Result == "SUCCESS" {
				return nil
			}
			return errors.Errorf("ANQP query failed (status=%q)", e.Result)
		}
	}
}

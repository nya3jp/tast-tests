// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
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
	names              []hostapd.VenueName
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
	// ANQP test aims to verify a DUT is able to fetch ANQP data from an access
	// point. A test access point is configured with a set of ANQP data, then
	// the DUT fetches it using fetch_anqp. The test obtains the data from the
	// DUT and verifies it is consistent with the access point configuration.
	testing.AddTest(&testing.Test{
		Func: ANQP,
		Desc: "Verifies that a DUT is able to perform ANQP requests and process replies",
		Contacts: []string{
			"damiendejean@chromium.org",       // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Timeout:     10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "anqp_basic_info",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoBar,
					names:              []hostapd.VenueName{{Lang: "eng", Name: "Foo bar"}},
					roamingConsortiums: []string{"ab56cd8971"},
					domains:            []string{"example.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_names",
				Val: anqpTestCase{
					infos: hostapd.VenueInfoZooOrAquarium,
					names: []hostapd.VenueName{
						{Lang: "eng", Name: "Local zoo park"},
						{Lang: "fra", Name: "Parc zoologique"},
					},
					roamingConsortiums: []string{"9836fc"},
					domains:            []string{"example.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_ois",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoHotelOrMotel,
					names:              []hostapd.VenueName{{Lang: "eng", Name: "My favorite hotel"}},
					roamingConsortiums: []string{"001bc50050", "001bc500b5"},
					domains:            []string{"example.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_domains",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoGasStation,
					names:              []hostapd.VenueName{{Lang: "eng", Name: "Foo station"}},
					roamingConsortiums: []string{"2233445566"},
					domains:            []string{"blue.net", "red.com", "green.com"},
					realms:             []hostapd.NAIRealm{tlsAndTtlsRealm},
				},
			}, {
				Name: "anqp_multiple_realms",
				Val: anqpTestCase{
					infos:              hostapd.VenueInfoAmphitheater,
					names:              []hostapd.VenueName{{Lang: "eng", Name: "Bar amphitheater"}},
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
	ssid               string
	ops                []hostapd.Option
	bssid              net.HardwareAddr
	venueInfo          hostapd.VenueInfo
	roamingConsortiums []string
	venueNames         []hostapd.VenueName
	domains            []string
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

	runner := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: s.DUT().Conn()})

	// Trigger a scan and wait for the network 'tc.ssid' to be available in
	// scan results.
	if err := runner.DiscoverNetwork(ctx, tf.DUTConn(wificell.DefaultDUT), tc.ssid); err != nil {
		s.Fatal("Network discovery failed: ", err)
	}

	// Ask wpa_supplicant to fetch ANQP data for all the compatible access point
	// found in range during last scan.
	if err := runner.FetchANQP(ctx, tf.DUTConn(wificell.DefaultDUT), tc.bssid.String()); err != nil {
		s.Fatal("ANQP fetch failed: ", err)
	}

	// Gather the data for the test access point.
	bss, err := runner.BSS(ctx, tc.bssid)
	if err != nil {
		s.Fatal("Failed to obtain BSS information: ", err)
	}

	// Check venue information match the test case.
	info, names, err := wifiutil.DecodeVenueGroupTypeName(bss["anqp_venue_name"])
	if err != nil {
		s.Fatal("Failed to parse venue info and names: ", err)
	}

	if !reflect.DeepEqual(info, tc.venueInfo) {
		s.Fatalf("Venue information does not match: got %v want %v", info, tc.venueInfo)
	}
	if !reflect.DeepEqual(names, tc.venueNames) {
		s.Fatalf("Venue names are not matching: got %v want %v", names, tc.venueNames)
	}

	// Check roaming consortium OIs are consistent with the test case.
	rcs, err := wifiutil.DecodeRoamingConsortiums(bss["anqp_roaming_consortium"])
	if err != nil {
		s.Fatal("Failed to parse roaming consortiums: ", err)
	}
	if !reflect.DeepEqual(rcs, tc.roamingConsortiums) {
		s.Fatalf("Romaing consortium OI are not matching: got %v want %v", rcs, tc.roamingConsortiums)
	}

	// Check domain names are gathered from the AP.
	domains, err := wifiutil.DecodeDomainNames(bss["anqp_domain_name"])
	if err != nil {
		s.Fatal("Failed to parse domain names: ", err)
	}
	if !reflect.DeepEqual(domains, tc.domains) {
		s.Fatalf("Domain names list are not matching: got %v want %v", domains, tc.domains)
	}

	// Check the realms are identical to the ones configured.
	realms, err := wifiutil.DecodeNAIRealms(bss["anqp_nai_realm"])
	if err != nil {
		s.Fatal("Failed to parse NAI Realms: ", err)
	}
	if !reflect.DeepEqual(realms, tc.realms) {
		s.Fatalf("Realms are not matching: got %v want %v", realms, tc.realms)
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

	ois := make(map[string]bool)
	for _, oi := range t.roamingConsortiums {
		ois[oi] = true
	}

	domains := make(map[string]bool)
	for _, domain := range t.domains {
		domains[domain] = true
	}

	return &anqpTestContext{
		ssid:               ssid,
		ops:                ops,
		bssid:              randAddr,
		venueInfo:          t.infos,
		venueNames:         t.names,
		roamingConsortiums: t.roamingConsortiums,
		domains:            t.domains,
		realms:             t.realms,
	}, nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"strings"

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        ProfileGUID,
		Desc:        "Verifies that shill correctly handles GUIDs in the context of WiFi services",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func ProfileGUID(fullCtx context.Context, s *testing.State) {
	const (
		guid      = "01234"
		password0 = "chromeos0"
		password1 = "chromeos1"
		password2 = "chromeos2"
	)

	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Error("Failed to tear down test fixture: ", err)
		}
	}()

	apCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ssid := hostapd.RandomSSID("TAST_TEST_")
	defer func() {
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ssid, err)
		}
	}()
	configureAPWithPassword := func(password string) (*wificell.APIface, error) {
		apOps := []hostapd.Option{
			hostapd.SSID(ssid),
			hostapd.Mode(hostapd.Mode80211b),
			hostapd.Channel(1),
		}
		secConfFac := wpa.NewConfigFactory(
			password,
			wpa.Mode(wpa.ModePureWPA),
			wpa.Ciphers(wpa.CipherCCMP),
		)
		return tf.ConfigureAP(apCtx, apOps, secConfFac)
	}

	func() {
		ap, err := configureAPWithPassword(password0)
		if err != nil {
			s.Fatal("Failed to configure ap: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(apCtx, ap); err != nil {
				s.Fatal("Failed to deconfig ap: ", err)
			}

		}()
		ctx, _ := tf.ReserveForDeconfigAP(apCtx, ap)

		props, err := genProps(ap.Config())
		if err != nil {
			s.Fatal("Failed to generate shill properties: ", err)
		}
		props[shillconst.ServicePropertyGUID] = guid
		propsEnc, err := protoutil.EncodeToShillValMap(props)
		if err != nil {
			s.Fatal("Failed to encode shill properties: ", err)
		}
		service, err := tf.WifiClient().ConfigureServiceAssertConnection(ctx,
			&network.ConfigureServiceAssertConnectionRequest{Props: propsEnc},
		)
		if err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}

		res, err := tf.WifiClient().QueryService(ctx, &network.QueryServiceRequest{Path: service.Path})
		if err != nil {
			s.Fatal("Failed to query service info: ", err)
		}
		if res.Guid != guid {
			s.Fatalf("GUID not match: got %q want %q", res.Guid, guid)
		}

		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx,
			&network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)},
		); err != nil {
			s.Fatalf("Failed to remove entries for ssid=%s: %v", ssid, err)
		}

		res, err = tf.WifiClient().QueryService(ctx, &network.QueryServiceRequest{Path: service.Path})
		if err != nil {
			s.Fatal("Failed to query service info: ", err)
		}
		if res.Guid != "" {
			s.Fatalf("Expected GUID not found, but got %q", res.Guid)
		}
	}()

	func() {
		ap, err := configureAPWithPassword(password1)
		if err != nil {
			s.Fatal("Failed to configure ap: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(apCtx, ap); err != nil {
				s.Fatal("Failed to deconfig ap: ", err)
			}

		}()
		ctx, _ := tf.ReserveForDeconfigAP(apCtx, ap)

		props, err := genProps(ap.Config())
		if err != nil {
			s.Fatal("Failed to generate shill properties: ", err)
		}
		props[shillconst.ServicePropertyGUID] = guid
		propsEnc, err := protoutil.EncodeToShillValMap(props)
		if err != nil {
			s.Fatal("Failed to encode shill properties: ", err)
		}
		service, err := tf.WifiClient().ConfigureServiceAssertConnection(ctx,
			&network.ConfigureServiceAssertConnectionRequest{Props: propsEnc},
		)
		if err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}

		res, err := tf.WifiClient().QueryService(ctx, &network.QueryServiceRequest{Path: service.Path})
		if err != nil {
			s.Fatal("Failed to query service info: ", err)
		}
		if res.Guid != guid {
			s.Fatalf("GUID not match: got %q want %q", res.Guid, guid)
		}
	}()

	func() {
		ap, err := configureAPWithPassword(password2)
		if err != nil {
			s.Fatal("Failed to configure ap: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(apCtx, ap); err != nil {
				s.Fatal("Failed to deconfig ap: ", err)
			}

		}()
		ctx, _ := tf.ReserveForDeconfigAP(apCtx, ap)

		propsEnc, err := protoutil.EncodeToShillValMap(map[string]interface{}{
			shillconst.ServicePropertyGUID:       guid,
			shillconst.ServicePropertyPassphrase: password2,
		})
		if err != nil {
			s.Fatal("Failed to encode shill properties: ", err)
		}
		if _, err := tf.WifiClient().ConfigureServiceAssertConnection(ctx,
			&network.ConfigureServiceAssertConnectionRequest{Props: propsEnc},
		); err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}
	}()
}

func genProps(conf *hostapd.Config) (map[string]interface{}, error) {
	props := map[string]interface{}{
		shillconst.ServicePropertyType:           shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID:    strings.ToUpper(hex.EncodeToString([]byte(conf.SSID))),
		shillconst.ServicePropertyWiFiHiddenSSID: conf.Hidden,
		shillconst.ServicePropertySecurityClass:  conf.SecurityConfig.Class(),
		shillconst.ServicePropertyAutoConnect:    true,
	}
	secProps, err := conf.SecurityConfig.ShillServiceProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shill security properties")
	}
	for k, v := range secProps {
		props[k] = v
	}
	return props, nil
}

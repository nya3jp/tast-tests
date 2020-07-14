// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"strings"

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
	assertGUID := func(ctx context.Context, expectedGUID string) error {
		res, err := tf.QueryService(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to query service info")
		}
		if res.Guid != expectedGUID {
			return errors.Errorf("GUID not match: got %q want %q", res.Guid, guid)
		}
		return nil
	}

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

		props, err := genShillProps(ap.Config())
		if err != nil {
			s.Fatal("Failed to generate shill properties: ", err)
		}
		props[shillconst.ServicePropertyGUID] = guid

		if err := tf.ConfigureServiceAssertConnection(ctx, props); err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}

		if err := assertGUID(ctx, guid); err != nil {
			s.Fatal("Failed on GUID assert: ", err)
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

		props := map[string]interface{}{
			shillconst.ServicePropertyGUID:       guid,
			shillconst.ServicePropertyPassphrase: password2,
		}
		if err := tf.ConfigureServiceAssertConnection(ctx, props); err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}

		if err := assertGUID(ctx, guid); err != nil {
			s.Fatal("Failed on GUID assert: ", err)
		}

		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx, req); err != nil {
			s.Fatalf("Failed to remove entries for ssid=%s: %v", ssid, err)
		}

		if err := assertGUID(ctx, ""); err != nil {
			s.Fatal("Failed on GUID assert: ", err)
		}
	}()
}

func genShillProps(conf *hostapd.Config) (map[string]interface{}, error) {
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

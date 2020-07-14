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
		Desc:        "Verifies that shill correctly handles GUIDs (Globally Unique IDentifier) in the context of WiFi services",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

const (
	guid      = "01234"
	password1 = "chromeos1"
	password2 = "chromeos2"
)

func ProfileGUID(fullCtx context.Context, s *testing.State) {
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

	func() {
		ap, err := configureAPWithPassword(apCtx, tf, ssid, password1)
		if err != nil {
			s.Fatal("Failed to configure ap: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(apCtx, ap); err != nil {
				s.Fatal("Failed to deconfig ap: ", err)
			}

		}()
		ctx, cancel := tf.ReserveForDeconfigAP(apCtx, ap)
		defer cancel()

		// Configure service with complete properties, including GUID.
		props, err := genShillProps(ap.Config())
		if err != nil {
			s.Fatal("Failed to generate shill properties: ", err)
		}
		servicePath, err := configureServiceAssertConnection(ctx, tf, props)
		if err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}

		if err := assertGUID(ctx, tf, servicePath, guid); err != nil {
			s.Fatal("Failed on GUID assert: ", err)
		}
	}()

	func() {
		ap, err := configureAPWithPassword(apCtx, tf, ssid, password2)
		if err != nil {
			s.Fatal("Failed to configure ap: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(apCtx, ap); err != nil {
				s.Fatal("Failed to deconfig ap: ", err)
			}

		}()
		ctx, cancel := tf.ReserveForDeconfigAP(apCtx, ap)
		defer cancel()

		// Change the password of the AP and modify only the password of the configuration with GUID.
		props := map[string]interface{}{
			shillconst.ServicePropertyGUID:       guid,
			shillconst.ServicePropertyPassphrase: password2,
		}
		servicePath, err := configureServiceAssertConnection(ctx, tf, props)
		if err != nil {
			s.Fatal("Failed to configure service and wait for connection: ", err)
		}

		if err := assertGUID(ctx, tf, servicePath, guid); err != nil {
			s.Fatal("Failed on GUID assert: ", err)
		}

		// Make sure that the GUID is missing after deleting the entries.
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(apCtx, req); err != nil {
			s.Fatalf("Failed to remove entries for ssid=%s: %v", ssid, err)
		}
		if err := assertGUID(ctx, tf, servicePath, ""); err != nil {
			s.Fatal("Failed on GUID assert: ", err)
		}
	}()
}

func configureAPWithPassword(ctx context.Context, tf *wificell.TestFixture, ssid, password string) (*wificell.APIface, error) {
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
	return tf.ConfigureAP(ctx, apOps, secConfFac)
}

func assertGUID(ctx context.Context, tf *wificell.TestFixture, servicePath, expectedGUID string) error {
	res, err := tf.WifiClient().QueryService(ctx, &network.QueryServiceRequest{Path: servicePath})
	if err != nil {
		return errors.Wrap(err, "failed to query service info")
	}
	if res.Guid != expectedGUID {
		return errors.Errorf("GUID not match: got %q want %q", res.Guid, guid)
	}
	return nil
}

func configureServiceAssertConnection(ctx context.Context, tf *wificell.TestFixture, props map[string]interface{}) (string, error) {
	propsEnc, err := protoutil.EncodeToShillValMap(props)
	if err != nil {
		return "", errors.Wrap(err, "failed to encode shill properties")
	}
	service, err := tf.WifiClient().ConfigureServiceAssertConnection(ctx,
		&network.ConfigureServiceAssertConnectionRequest{Props: propsEnc},
	)
	if err != nil {
		return "", err
	}
	return service.Path, nil
}

func genShillProps(conf *hostapd.Config) (map[string]interface{}, error) {
	props := map[string]interface{}{
		shillconst.ServicePropertyGUID:           guid,
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

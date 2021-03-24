// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MTBF,
		Desc: "Run typical WiFi use cases and measure the Mean Time Between Failures (MTBF)",
		Contacts: []string{
			"chharry@google.com",              // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_mtbf", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		Timeout:     5 * time.Hour,
	})
}

func MTBF(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	const (
		pingDuration = 5 * time.Minute
		pingInterval = 0.5 // seconds
		// TODO(b/184596754): Find a way to suspend for longer time.
		suspendDuration = 2 * time.Minute

		runOnceTimeout = 2*time.Minute + // Misc configuring and cleanup
			pingDuration + // Simple Connect
			suspendDuration + pingDuration + // Suspend and reconnect to AP1
			suspendDuration + pingDuration + // Suspend and move to AP2
			suspendDuration + pingDuration // Suspend and move back to AP1

		waitSuspendTimeout = 10 * time.Second

		ap1SSIDPrefix = "TAST_WIFI_MTBF_AP1_"
		ap2SSIDPrefix = "TAST_WIFI_MTBF_AP2_"
	)

	eapCert1 := certificate.TestCert1()

	ap1Config := func() ([]hostapd.Option, security.ConfigFactory) {
		mac, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to generate a random MAC address: ", err)
		}
		ssid := hostapd.RandomSSID(ap1SSIDPrefix)
		opts := []hostapd.Option{
			hostapd.SSID(ssid), hostapd.BSSID(mac.String()),
			hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1),
			hostapd.HTCaps(hostapd.HTCapHT20),
		}
		secConfFac := wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP))
		return opts, secConfFac
	}
	ap2Config := func() ([]hostapd.Option, security.ConfigFactory) {
		mac, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to generate a random MAC address: ", err)
		}
		ssid := hostapd.RandomSSID(ap2SSIDPrefix)
		opts := []hostapd.Option{
			hostapd.SSID(ssid), hostapd.BSSID(mac.String()),
			hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(157),
			hostapd.HTCaps(hostapd.HTCapHT40Plus),
			hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155),
		}
		secConfFac := wpaeap.NewConfigFactory(
			eapCert1.CACred.Cert, eapCert1.ServerCred,
			wpaeap.ClientCACert(eapCert1.CACred.Cert),
			wpaeap.ClientCred(eapCert1.ClientCred),
		)
		return opts, secConfFac
	}

	// TODO(b/184599948): Determine the traffic type and pass/fail criteria.
	traffic := func(ctx context.Context, ip net.IP) error {
		s.Log("Start pinging for ", pingDuration)
		return tf.PingFromDUT(ctx, ip.String(),
			ping.Count(int(pingDuration.Seconds()/pingInterval)),
			ping.Interval(pingInterval),
		)
	}

	configureAP := func(ctx context.Context, opts []hostapd.Option, secConfFac security.ConfigFactory) *wificell.APIface {
		ap, err := tf.ConfigureAP(ctx, opts, secConfFac)
		if err != nil {
			s.Fatal("Failed to start the AP: ", err)
		}
		return ap
	}
	deconfigAP := func(ctx context.Context, ap **wificell.APIface) {
		if *ap != nil {
			if err := tf.DeconfigAP(ctx, *ap); err != nil {
				s.Fatal("Failed to deconfigure the AP: ", err)
			}
			*ap = nil
		}
	}

	bgSuspend := func(ctx context.Context, duration time.Duration) (_ <-chan error, cleanup func()) {
		ch := make(chan error, 1)
		bgCtx, cancel := context.WithCancel(ctx)
		go func() {
			defer close(ch)
			ch <- tf.Suspend(bgCtx, duration)
		}()
		return ch, func() {
			// In case we failed and returned early in fg routine, cancel the bg routine and wait for it.
			cancel()
			<-ch
		}
	}

	runOnce := func(ctx context.Context) (succeeded bool) {
		ap1Opts, ap1SecConfFac := ap1Config()
		ap2Opts, ap2SecConfFac := ap2Config()

		// Reinit the DUT after each round.
		defer func(ctx context.Context) {
			res, err := tf.WifiClient().SelectedService(ctx, &empty.Empty{})
			if err != nil {
				s.Log("Failed to get the selected service: ", err)
				succeeded = false
				// Try to reinit the DUT even if we can't get the selected service.
				if err := tf.Reinit(ctx); err != nil {
					s.Log("Failed to reinit the DUT: ", err)
				}
				return
			}

			propsRecv, err := tf.ExpectShillProperty(ctx, res.ServicePath, []*wificell.ShillProperty{{
				Property:       shillconst.ServicePropertyState,
				ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
				Method:         network.ExpectShillPropertyRequest_CHECK_WAIT,
			}}, nil)
			if err != nil {
				s.Log("Failed to start shill property watcher: ", err)
				succeeded = false
			}
			if err := tf.Reinit(ctx); err != nil {
				s.Log("Failed to reinit the DUT: ", err)
				succeeded = false
			}
			if _, err := propsRecv(); err != nil {
				s.Log("Failed to wait for the DUT enter idle state after reiniting: ", err)
				succeeded = false
			}
		}(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
		defer cancel()

		s.Log("Step: Simple connect to AP1")
		ap1 := configureAP(ctx, ap1Opts, ap1SecConfFac)
		defer deconfigAP(ctx, &ap1)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()
		ap1Conn, err := tf.ConnectWifiAP(ctx, ap1)
		if err != nil {
			s.Log("Failed to connect to AP1: ", err)
			return false
		}
		if err := traffic(ctx, ap1.ServerIP()); err != nil {
			s.Log("Failed to generate traffics to AP1: ", err)
			return false
		}

		s.Log("Step: Suspend and reconnect to AP1")
		if _, err := tf.SuspendAssertConnect(ctx, suspendDuration); err != nil {
			s.Log("Failed to suspend and reconnect to AP1: ", err)
			return false
		}
		if err := traffic(ctx, ap1.ServerIP()); err != nil {
			s.Log("Failed to generate traffics to AP1: ", err)
			return false
		}

		s.Log("Step: Suspend and move to AP2")
		disconnFromAp1Props, err := tf.ExpectShillProperty(ctx, ap1Conn.ServicePath, []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		}}, nil)
		if err != nil {
			s.Log("Failed to start shill property watcher for disconnect from AP1: ", err)
			return false
		}
		suspendErr, cleanup := bgSuspend(ctx, suspendDuration)
		defer cleanup()

		// Wait for the DUT to fall asleep before changing AP config.
		susCtx, cancel := context.WithTimeout(ctx, waitSuspendTimeout)
		defer cancel()
		if err := s.DUT().WaitUnreachable(susCtx); err != nil {
			s.Log("Failed to wait for the DUT become unreachable: ", err)
			return false
		}
		deconfigAP(ctx, &ap1)
		ap2 := configureAP(ctx, ap2Opts, ap2SecConfFac)
		defer deconfigAP(ctx, &ap2)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
		defer cancel()

		s.Log("AP setup done; Waiting for the suspend routine")
		if err := <-suspendErr; err != nil {
			s.Log("Failed to suspend: ", err)
			return false
		}
		if _, err := disconnFromAp1Props(); err != nil {
			s.Log("Failed to disconnect from AP1: ", err)
			return false
		}

		if _, err := tf.ConnectWifiAP(ctx, ap2); err != nil {
			s.Log("Failed to connect to AP2: ", err)
			return false
		}
		if err := traffic(ctx, ap2.ServerIP()); err != nil {
			s.Log("Failed to generate traffics to AP2: ", err)
			return false
		}

		s.Log("Step: Suspend and move back to AP1")
		backToAP1Props, err := tf.ExpectShillProperty(ctx, ap1Conn.ServicePath, []*wificell.ShillProperty{{
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
			Method:         network.ExpectShillPropertyRequest_CHECK_ONLY,
		}, {
			Property:       shillconst.ServicePropertyIsConnected,
			ExpectedValues: []interface{}{true},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		}}, nil)
		if err != nil {
			s.Log("Failed to start shill property watcher for reconnect to AP1: ", err)
			return false
		}
		suspendErr, cleanup = bgSuspend(ctx, suspendDuration)
		defer cleanup()

		// Wait for the DUT to fall asleep before changing AP config.
		susCtx, cancel = context.WithTimeout(ctx, waitSuspendTimeout)
		defer cancel()
		if err := s.DUT().WaitUnreachable(susCtx); err != nil {
			s.Log("Failed to wait for the DUT become unreachable: ", err)
			return false
		}
		deconfigAP(ctx, &ap2)
		ap1 = configureAP(ctx, ap1Opts, ap1SecConfFac)

		s.Log("AP setup done; Waiting for the suspend routine")
		if err := <-suspendErr; err != nil {
			s.Log("Failed to suspend: ", err)
			return false
		}
		if _, err := backToAP1Props(); err != nil {
			s.Log("Failed to reconnect to AP1: ", err)
			return false
		}
		if err := traffic(ctx, ap1.ServerIP()); err != nil {
			s.Log("Failed to generate traffics to AP1: ", err)
			return false
		}

		return true
	}

	startTS := time.Now()
	for {
		runOnceCtx, cancel := context.WithTimeout(ctx, runOnceTimeout)
		defer cancel()
		succeeded := runOnce(runOnceCtx)
		mtbf := time.Now().Sub(startTS)
		if !succeeded {
			s.Log("MTBF failed, mean time=", mtbf)
			return
		}
		if d, _ := ctx.Deadline(); time.Now().Add(runOnceTimeout).After(d) {
			// No enough time for the next round.
			s.Log("MTBF passed, mean time=", mtbf)
			return
		}
	}
}

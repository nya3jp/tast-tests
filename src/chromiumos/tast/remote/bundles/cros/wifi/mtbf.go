// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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
		Fixture:     "wificellFixt",
		Timeout:     5 * time.Hour,
	})
}

func MTBF(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	const (
		pingDuration = 5 * time.Minute
		pingInterval = 0.5 // seconds
		// TODO(b/184596754): Find a way to suspend for longer time.
		suspendDuration = 2 * time.Minute

		simpleConnectTimeout    = pingDuration + time.Minute
		suspendReconnectTimeout = suspendDuration + pingDuration + time.Minute
		suspendMoveTimeout      = suspendDuration + pingDuration + time.Minute
		suspendMoveBackTimeout  = suspendReconnectTimeout

		runOnceTimeout = simpleConnectTimeout + suspendReconnectTimeout + suspendMoveTimeout + suspendMoveBackTimeout +
			time.Minute // Misc configuring and cleanup

		waitSuspendTimeout = 10 * time.Second

		timeForReinit = time.Second * 10

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

	bgSuspend := func(ctx context.Context, duration time.Duration) (suspendErrCh <-chan error, cleanup func()) {
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

	// Vars that record the iteration info of the MTBF test and will also be reset/updated in runOnce.
	var (
		run  = 1
		step = 1
	)
	runStep := func(ctx context.Context, desc string, stepTimeout time.Duration, f func(ctx context.Context) error) (succeeded bool) {
		defer func() { step++ }()
		ctx, cancel := context.WithTimeout(ctx, stepTimeout)
		defer cancel()
		s.Logf("Run %d: Step %d: %v", run, step, desc)
		ctx, st := timing.Start(ctx, desc)
		defer st.End()
		if err := f(ctx); err != nil {
			s.Logf("Failed at run %d: step %d: %v, err=%v", run, step, desc, err)
			return false
		}
		return true
	}

	runOnce := func(ctx context.Context) (succeeded bool) {
		defer func() {
			run++
			step = 1
		}()

		ap1Opts, ap1SecConfFac := ap1Config()
		ap2Opts, ap2SecConfFac := ap2Config()

		// Reinit the DUT after each round.
		defer func(ctx context.Context) {
			if err := tf.Reinit(ctx); err != nil {
				s.Log("Failed to reinit the DUT: ", err)
				succeeded = false
			}
		}(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, timeForReinit)
		defer cancel()

		ap1 := configureAP(ctx, ap1Opts, ap1SecConfFac)
		defer deconfigAP(ctx, &ap1)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()
		// Schedule defer for AP2 before configuring to make sure
		// the AP will be cleaned up after each round.
		var ap2 *wificell.APIface
		defer deconfigAP(ctx, &ap2)
		// Borrow the reserve of AP1 as the AP2 is not yet configured.
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()

		var ap1ServicePath string
		if !runStep(ctx, "Connect to AP1 and perform ping test", simpleConnectTimeout, func(ctx context.Context) error {
			ap1Conn, err := tf.ConnectWifiAP(ctx, ap1)
			ap1ServicePath = ap1Conn.ServicePath
			if err != nil {
				return errors.Wrap(err, "failed to connect to AP1")
			}
			if err := traffic(ctx, ap1.ServerIP()); err != nil {
				return errors.Wrap(err, "failed to generate traffics to AP1")
			}
			return nil
		}) {
			return false
		}

		if !runStep(ctx, fmt.Sprintf("Suspend for %v and reconnect to AP1", suspendDuration), suspendReconnectTimeout, func(ctx context.Context) error {
			if _, err := tf.SuspendAssertConnect(ctx, suspendDuration); err != nil {
				return errors.Wrap(err, "failed to suspend and reconnect to AP1")
			}
			if err := traffic(ctx, ap1.ServerIP()); err != nil {
				return errors.Wrap(err, "failed to generate traffics to AP1")
			}
			return nil
		}) {
			return false
		}

		if !runStep(ctx, fmt.Sprintf("Suspend for %v and move to AP2", suspendDuration), suspendMoveTimeout, func(ctx context.Context) error {
			disconnFromAp1Props, err := tf.ExpectShillProperty(ctx, ap1ServicePath, []*wificell.ShillProperty{{
				Property:       shillconst.ServicePropertyState,
				ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
				Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
			}}, nil)
			if err != nil {
				return errors.Wrap(err, "failed to start shill property watcher for disconnect from AP1")
			}
			suspendErrCh, cleanup := bgSuspend(ctx, suspendDuration)
			defer cleanup()

			// Wait for the DUT to fall asleep before changing AP config.
			susCtx, cancel := context.WithTimeout(ctx, waitSuspendTimeout)
			defer cancel()
			if err := s.DUT().WaitUnreachable(susCtx); err != nil {
				return errors.Wrap(err, "failed to wait for the DUT become unreachable")
			}
			deconfigAP(ctx, &ap1)
			ap2 = configureAP(ctx, ap2Opts, ap2SecConfFac)

			s.Log("AP setup done; Waiting for the suspend routine")
			if err := <-suspendErrCh; err != nil {
				return errors.Wrap(err, "failed to suspend")
			}
			if _, err := disconnFromAp1Props(); err != nil {
				return errors.Wrap(err, "failed to disconnect from AP1")
			}

			if _, err := tf.ConnectWifiAP(ctx, ap2); err != nil {
				return errors.Wrap(err, "failed to connect to AP2")
			}
			if err := traffic(ctx, ap2.ServerIP()); err != nil {
				return errors.Wrap(err, "failed to generate traffics to AP2")
			}
			return nil
		}) {
			return false
		}

		return runStep(ctx, fmt.Sprintf("Suspend for %v and move back to AP1", suspendDuration), suspendMoveBackTimeout, func(ctx context.Context) error {
			backToAP1Props, err := tf.ExpectShillProperty(ctx, ap1ServicePath, []*wificell.ShillProperty{{
				Property:       shillconst.ServicePropertyState,
				ExpectedValues: []interface{}{shillconst.ServiceStateIdle},
				Method:         wifi.ExpectShillPropertyRequest_CHECK_ONLY,
			}, {
				Property:       shillconst.ServicePropertyIsConnected,
				ExpectedValues: []interface{}{true},
				Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
			}}, nil)
			if err != nil {
				return errors.Wrap(err, "failed to start shill property watcher for reconnect to AP1")
			}
			suspendErrCh, cleanup := bgSuspend(ctx, suspendDuration)
			defer cleanup()

			// Wait for the DUT to fall asleep before changing AP config.
			susCtx, cancel := context.WithTimeout(ctx, waitSuspendTimeout)
			defer cancel()
			if err := s.DUT().WaitUnreachable(susCtx); err != nil {
				return errors.Wrap(err, "failed to wait for the DUT become unreachable")
			}
			deconfigAP(ctx, &ap2)
			ap1 = configureAP(ctx, ap1Opts, ap1SecConfFac)

			s.Log("AP setup done; Waiting for the suspend routine")
			if err := <-suspendErrCh; err != nil {
				return errors.Wrap(err, "failed to suspend")
			}
			if _, err := backToAP1Props(); err != nil {
				return errors.Wrap(err, "failed to reconnect to AP1")
			}
			if err := traffic(ctx, ap1.ServerIP()); err != nil {
				return errors.Wrap(err, "failed to generate traffics to AP1")
			}
			return nil
		})
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
		d, ok := ctx.Deadline()
		if !ok {
			s.Fatal("No deadline is set to the context")
		}
		if time.Now().Add(runOnceTimeout).After(d) {
			// No enough time for the next round.
			s.Log("MTBF passed, mean time=", mtbf)
			return
		}
	}
}

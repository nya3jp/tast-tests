// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Reassociate,
		Desc:        "Timing test for wpa_supplicant reassociate operation",
		Contacts:    []string{"wgd@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func Reassociate(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to tear down test fixture: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	// CR: Do we want performance data or is this test just pass/fail?
	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
		}
	}()

	ap, err := tf.ConfigureAP(ctx, []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6)}, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	s.Log("AP Configured")

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for SSID=%s: %v", ap.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	s.Log("Connected to AP")

	logger, err := iw.NewEventLogger(ctx, s.DUT())
	if err != nil {
		s.Error("Failed to start event logger: ", err)
	}
	defer func(ctx context.Context) {
		if err := logger.Stop(ctx); err != nil {
			s.Error("Failed to stop event logger: ", err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	s.Log("Started event logger")

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Error("Failed to get WiFi interface: ", err)
	}
	if err := runWPACommand(ctx, s.DUT().Conn(), iface, "reassociate"); err != nil {
		s.Error("Failed to run reassociate command: ", err)
	}
	s.Log("Reassociating")

	dt, err := getReassociationTime(ctx, logger, 10*time.Second)
	if err != nil {
		s.Error("Reassociation (or timing measurement) failed: ", err)
	}
	s.Log("Reassociated after: ", dt)

	// CR: Again, do we actually want this or is pass/fail sufficient?
	pv.Set(perf.Metric{
		Name:      "reassociation_time",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, dt.Seconds())
}

func runWPACommand(ctx context.Context, host *ssh.Conn, ifname, command string) error {
	// CR: Is it acceptable to directly run the command or should there
	// be a `wpa_cli` abstraction library? Do we need to support all the
	// platforms the original Python version supported (Android/Brillo/
	// ChromeOS/Cast) or is just ChromeOS sufficient here?
	r := &cmd.RemoteCmdRunner{Host: host}
	c := fmt.Sprintf("/usr/bin/wpa_cli -i %s %s", ifname, command)
	return r.Run(ctx, "su", "wpa", "-s", "/bin/bash", "-c", c)
}

func getReassociationTime(ctx context.Context, logger *iw.EventLogger, timeout time.Duration) (time.Duration, error) {
	var dt time.Duration
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		discs := logger.EventsByType(iw.EventTypeDisconnect)
		if len(discs) == 0 {
			return errors.New("not yet disconnected")
		}
		conns := logger.EventsByType(iw.EventTypeConnect)
		if len(conns) == 0 {
			return errors.New("not yet reconnected")
		}
		if len(discs) > 1 || len(conns) > 1 {
			return testing.PollBreak(errors.New("multiple disconnections or connections logged"))
		}
		dt = conns[0].Timestamp.Sub(discs[0].Timestamp)
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return 0, err
	}
	return dt, nil
}

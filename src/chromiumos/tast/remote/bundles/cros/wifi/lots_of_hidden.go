// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// *
// *
// *
// *
// *
// ************************************************************************
// BIG WARNING: this spins up a ton of SSH sessions on the AP. The current
// default MaxSessions is somewhat low, so one needs to toss something like
// "MaxSessions 100" into /etc/ssh/sshd_config on the AP for this to work.
// ************************************************************************
// *
// *
// *
// *
// *

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LotsOfHidden,
		Desc: "Spin up a bunch of hidden SSID networks",
		Contacts: []string{
			"briannorris@chromium.org",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

func LotsOfHidden(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	configureAP := func(ctx context.Context, channel int) (context.Context, *wificell.APIface, func(context.Context), error) {
		s.Logf("Setting up the AP on channel %d", channel)
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.Hidden()}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return ctx, nil, nil, err
		}
		sCtx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		deferFunc := func(ctx context.Context) {
			s.Logf("Deconfiguring the AP on channel %d", channel)
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
			cancel()
		}
		return sCtx, ap, deferFunc, nil
	}

	// *
	// *
	// *
	// *
	// *
	// See the BIG WARNING above.
	// *
	// *
	// *
	// *
	// *
	// Spin up 32 APs, 16 on each band  -- 2.4GHz (channel 1) and 5GHz (channel 36).
	var aps []*wificell.APIface
	for _, ch := range []int{1, 36} {
		for i := 0; i < 16; i++ {
			sCtx, ap, deconfig, err := configureAP(ctx, ch)
			if err != nil {
				s.Fatalf("Failed to set up AP #%d, channel %d: %v", i, ch, err)
			}
			defer deconfig(ctx)
			aps = append(aps, ap)
			ctx = sCtx
		}
	}

	s.Log("Still figuring out what to do here...so sleeping 60 seconds instead, for debug")
	testing.Sleep(ctx, time.Minute)
	s.Log("OK, we're done")
}

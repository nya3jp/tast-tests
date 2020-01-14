// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Finalize,
		Desc:     "Test finalize process in factory toolkit",
		Contacts: []string{"menghuan@chromium.org", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func Finalize(fullCtx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(fullCtx, time.Minute)
	defer cancel()
	defer cleanup(ctx, s)

	d := s.DUT()

	// "gooftool" of "factory-mini" package has been already installed on test image.
	if err := d.Command("gooftool", "wipe_in_place", "--test_umount").Run(ctx); err != nil {
		s.Fatal("Failed to run wiping of finalize: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := d.Command("cat", "/tmp/wipe_init.log").Output(ctx)
		if err != nil {
			return errors.Wrap(err, "fail to access log")
		}

		if !strings.Contains(string(out), "GOOFTOOL command 'wipe_init' SUCCESS") {
			return errors.New("wipe have not finished yet")
		}

		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to wait wiping finished: ", err)
	}
}

func cleanup(ctx context.Context, s *testing.State) {
	s.Log("start to cleanup")

	d := s.DUT()

	// Reboot to recover umounted partitiions.
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}

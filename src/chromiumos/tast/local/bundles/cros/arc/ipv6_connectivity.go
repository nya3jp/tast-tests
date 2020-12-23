// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IPv6Connectivity,
		Desc:     "Checks IPv6 connectivity inside ARC",
		Contacts: []string{"taoyl@google.com", "cros-networking@google.com"},
		// FIXME: Uncomment Attr: after lab supports IPv6.  http://b/161239666
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
	})
}

func IPv6Connectivity(ctx context.Context, s *testing.State) {
	const (
		pingTimeout      = 10 * time.Second
		googleDNSIPv6    = "2001:4860:4860::8888"
		googleDotComIPv6 = "ipv6.google.com"
	)
	a := s.FixtValue().(*arc.PreData).ARC

	verify := func(ctx context.Context, cmd func(context.Context, string, ...string) *testexec.Cmd, bindir string) error {
		// Verify global IPv6 address is configured correctly.
		out, err := cmd(ctx, filepath.Join(bindir, "ip"), "-6", "addr", "show", "scope", "global").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to get address information")
		}
		if len(out) == 0 {
			return errors.New("no global IPv6 address is configured")
		}
		// Verify connectivity to literal IPv6 address destination.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return cmd(ctx, filepath.Join(bindir, "ping6"), "-c1", "-w1", googleDNSIPv6).Run()
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			return errors.Wrapf(err, "cannot ping %s", googleDNSIPv6)
		}
		// Verify connectivity to IPv6-only host name.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return cmd(ctx, filepath.Join(bindir, "ping6"), "-c1", "-w1", googleDotComIPv6).Run()
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			return errors.Wrapf(err, "cannot ping %s", googleDotComIPv6)
		}
		return nil
	}

	// Check IPv6 availablility at host first. If test fails in this part then it's a lab net issue instead of ARC issue.
	if err := verify(ctx, testexec.CommandContext, "/bin"); err != nil {
		s.Fatal("Failed to communicate from host, check lab network setting: ", err)
	}

	if err := verify(ctx, a.Command, "/system/bin"); err != nil {
		s.Error("Failed to communicate from ARC: ", err)
	}

}

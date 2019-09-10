// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MulticastForwarder,
		Desc:         "Checks multicast forwarder works on Android",
		Contacts:     []string{"jasongustaman@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcMulticastForwarderTest.apk"},
		Pre:          arc.Booted(),
	})
}

func MulticastForwarder(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcMulticastForwarderTest.apk"
		pkg = "org.chromium.arc.testapp.multicast_forwarder"
		cls = "org.chromium.arc.testapp.multicast_forwarder.MulticastForwarderActivity"

		mdnsButtonID       = "org.chromium.arc.testapp.multicast_forwarder:id/button_mdns"
		legacyMdnsButtonID = "org.chromium.arc.testapp.multicast_forwarder:id/button_legacy_mdns"
		ssdpButtonID       = "org.chromium.arc.testapp.multicast_forwarder:id/button_ssdp"
		hostnameID         = "org.chromium.arc.testapp.multicast_forwarder:id/hostname"
		portID             = "org.chromium.arc.testapp.multicast_forwarder:id/port"

		// Addresses of multicast protocol.
		mdnsAddr = "224.0.0.251"
		ssdpAddr = "239.255.255.250"

		// Random hostname and port used to verify if packet successfully forwarded.
		mdnsHostname       = "bff40af49dd97a3d1951f5af9c2b648099f15bbb.local"
		legacyMdnsHostname = "a2ffec2cb85be5e7be43b2b0f4b7187379347e27.local"
		legacyMdnsPort     = "10101"
		ssdpPort           = "9191"

		// Timeout and retry constant.
		timeout = 30 * time.Second
		retry   = 5
	)

	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	if err := d.Object(ui.ID(mdnsButtonID)).WaitForExists(ctx, timeout); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// Create a shorter context for inbound traffic check.
	watchCtx, watchCancel := context.WithTimeout(ctx, timeout)
	defer watchCancel()

	g, watchCtx := errgroup.WithContext(watchCtx)

	// Start tcpdump for mDNS.
	g.Go(func() error {
		if err := watchTcpdump(watchCtx, mdnsHostname, "", mdnsAddr); err != nil {
			return errors.Errorf("failed to get mDNS packet %s", err)
		}
		return nil
	})
	// Start tcpdump for legacy mDNS.
	g.Go(func() error {
		if err := watchTcpdump(watchCtx, legacyMdnsHostname, legacyMdnsPort, mdnsAddr); err != nil {
			return errors.Errorf("failed to get legacy mDNS packet %s", err)
		}
		return nil
	})
	// Start tcpdump for SSDP.
	g.Go(func() error {
		if err := watchTcpdump(watchCtx, "", ssdpPort, ssdpAddr); err != nil {
			return errors.Errorf("failed to get SSDP packet %s", err)
		}
		return nil
	})

	setTexts := func(hostname, port string) {
		if err := d.Object(ui.ID(hostnameID)).SetText(ctx, hostname); err != nil {
			s.Error("Failed setting hostname: ")
		}
		if err := d.Object(ui.ID(hostnameID), ui.Focused(true)).WaitForExists(ctx, timeout); err != nil {
			s.Errorf("Failed to focus on field %s", hostnameID)
		}

		if err := d.Object(ui.ID(portID)).SetText(ctx, port); err != nil {
			s.Error("Failed setting port: ", err)
		}
		if err := d.Object(ui.ID(portID), ui.Focused(true)).WaitForExists(ctx, timeout); err != nil {
			s.Errorf("Failed to focus on field %s", portID)
		}
	}

	// Try to send multicast packet multiple times.
	for i := 0; i < retry; i++ {
		// Run mDNS query.
		setTexts(mdnsHostname, "")
		if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
			s.Error("Failed starting mDNS test: ", err)
		}

		// Run legacy mDNS query.
		setTexts(legacyMdnsHostname, legacyMdnsPort)
		if err := d.Object(ui.ID(legacyMdnsButtonID)).Click(ctx); err != nil {
			s.Error("Failed starting legacy mDNS test: ", err)
		}

		// Run SSDP query
		setTexts("", ssdpPort)
		if err := d.Object(ui.ID(ssdpButtonID)).Click(ctx); err != nil {
			s.Error("Failed starting SSDP test: ", err)
		}
	}

	if err := g.Wait(); err != nil {
		s.Fatal("Failed multicast forwarding egress check: ", err)
	}
}

func watchTcpdump(ctx context.Context, hostname, port, addr string) error {
	// Starts a tcpdump process that writes messages to stdout on new network messages.
	tcpdump := testexec.CommandContext(ctx, "/usr/local/sbin/tcpdump", "-ni", "any", "-P", "out")

	stdout, err := tcpdump.StdoutPipe()
	if err != nil {
		return err
	}

	if err := tcpdump.Start(); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}

	// sc.Scan() below might block. Release bufio.Scanner by killing tcpdump if the
	// process execution time exceeds context deadline.
	go func() {
		defer tcpdump.Wait()
		defer tcpdump.Kill()

		// Blocks until deadline is passed.
		<-ctx.Done()
	}()

	// Watch and wait until tcpdump output the correct src and dst with hostname.
	sc := bufio.NewScanner(stdout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if sc.Scan() {
			text := sc.Text()
			if hostname != "" && !strings.Contains(text, hostname) {
				continue
			}
			if !strings.Contains(text, port+" > "+addr) {
				continue
			}
			return nil
		}
	}
}

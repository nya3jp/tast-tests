// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/mdns"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/p2p"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     P2PClient,
		Desc:     "Tests that Chromium OS can download files from local network peers with p2p-client",
		Contacts: []string{"ahassani@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

type fakeP2PServer struct {
	server  *mdns.Server
	service *mdns.MDNSService
}

func newFakeP2PServer(port, numConn int, files map[string]int) (*fakeP2PServer, error) {
	instance := fmt.Sprintf("instance%d", port)

	txt := []string{fmt.Sprintf("num_connections=%d", numConn)}
	for name, size := range files {
		txt = append(txt, fmt.Sprintf("id_%s=%d", name, size))
	}

	service, err := mdns.NewMDNSService(instance, p2p.ServiceType, "", "", port, []net.IP{net.ParseIP(p2p.DefaultNSIP)}, txt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create mDNS service")
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, errors.Wrap(err, "failed to start mDNS server")
	}

	s := &fakeP2PServer{server, service}
	return s, nil
}

func (s *fakeP2PServer) close() error {
	return s.server.Shutdown()
}

func P2PClient(fullCtx context.Context, s *testing.State) {
	// Shorten the timeout to allow some time for cleanup.
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	if err := p2p.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up: ", err)
	}
	defer func() {
		if err := p2p.CleanUp(fullCtx); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}()

	const (
		port1 = 1111
		port2 = 2222
		port3 = 3333
		port4 = 4444
	)

	s.Log("Starting fake servers")

	srv1, err := newFakeP2PServer(port1, 1, map[string]int{"everyone": 1000, "only-a": 5000})
	if err != nil {
		s.Fatalf("Failed to start a fake server with port %d: %v", port1, err)
	}
	defer srv1.close()

	srv2, err := newFakeP2PServer(port2, 0, map[string]int{"everyone": 10000, "only-b": 8000})
	if err != nil {
		s.Fatalf("Failed to start a fake server with port %d: %v", port2, err)
	}
	defer srv2.close()

	srv3, err := newFakeP2PServer(port3, 1, map[string]int{"everyone": 20000})
	if err != nil {
		s.Fatalf("Failed to start a fake server with port %d: %v", port3, err)
	}
	defer srv3.close()

	// Wait for all fake servers to be found by avahi.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "p2p-client", "--list-all")
		out, err := cmd.Output()
		if err != nil {
			return errors.Wrap(err, "failed to enumerate servers")
		}
		s := string(out)
		for _, port := range []int{port1, port2, port3} {
			if !strings.Contains(s, fmt.Sprintf("port %d", port)) {
				return errors.Errorf("fake server at port %d is not found", port)
			}
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for fake servers to be found by avahi: ", err)
	}

	// Request a file shared from only one peer.
	s.Log("Querying only-a")
	cmd := testexec.CommandContext(ctx, "p2p-client", "--get-url=only-a")
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("p2p-client failed to query only-a: ", err)
	} else if got, want := strings.TrimSpace(string(out)), fmt.Sprintf("http://%s:%d/only-a", p2p.DefaultNSIP, port1); got != want {
		s.Errorf("p2p-client failed to query only-a; got %s, want %s", got, want)
	}

	// Check that the num_connections is reported properly.
	s.Log("Counting connections")
	cmd = testexec.CommandContext(ctx, "p2p-client", "--num-connections")
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("p2p-client --num-connections failed: ", err)
	} else if got := strings.TrimSpace(string(out)); got != "2" {
		s.Errorf("p2p-client --num-connections failed; got %s, want 2", got)
	}

	// Request a file shared from a peer with enough of the file.
	s.Log("Querying everyone with --minimum-size=15000")
	cmd = testexec.CommandContext(ctx, "p2p-client", "--get-url=everyone", "--minimum-size=15000")
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("p2p-client failed to query everyone: ", err)
	} else if got, want := strings.TrimSpace(string(out)), fmt.Sprintf("http://%s:%d/everyone", p2p.DefaultNSIP, port3); got != want {
		s.Errorf("p2p-client failed to query everyone; got %s, want %s", got, want)
	}

	// Request too many bytes of an existing file.
	s.Log("Querying only-b with --minimum-size=10000")
	cmd = testexec.CommandContext(ctx, "p2p-client", "--get-url=only-b", "--minimum-size=10000")
	if err := cmd.Run(); err == nil {
		cmd.DumpLog(ctx)
		s.Error("p2p-client succeeded querying only-b; expected to fail")
	}

	// Check that p2p-client hangs while waiting for a peer when there are too many connections.
	s.Log("Adding a new fake server with large connection count")
	srv4, err := newFakeP2PServer(port4, 98, map[string]int{"everyone": 10000})
	if err != nil {
		s.Fatalf("Failed to start a fake server with port %d: %v", port4, err)
	}
	defer srv4.close()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd = testexec.CommandContext(ctx, "p2p-client", "--num-connections")
		if out, err := cmd.Output(); err != nil {
			return errors.Wrap(err, "failed to count connections")
		} else if got := strings.TrimSpace(string(out)); got != "100" {
			return errors.Errorf("got %s, want 100", got)
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for connection count update: ", err)
	}

	s.Log("Querying only-b")
	shortCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := testexec.CommandContext(shortCtx, "p2p-client", "--get-url=only-b").Run(); err == nil {
		s.Fatal("p2p-client finished, but should have waited for num_connections to drop")
	}
}

// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package p2p contains the common code for P2P tests.
//
// The most challenging part of P2P tests is that the P2P protocol uses
// multicasts for DNS-SD, but multicasts do not work properly with loopback
// network interfaces.
//
// To tackle with the problem, we use network namespaces. We set up an isolated
// network namespace named "tastns", create virtual network interface pair
// connecting tastns and the default network namespace, and run avahi-daemon in
// tastns. Tast test process runs in the default namespace. This way we can make
// sure multicast packets go through routing instead of loopback.
//
// Note that p2p_server and p2p_client also run in the default namespace, but
// they communicate with avahi-daemon running in tastns via D-Bus IPC, so they
// work just as if they are in tastns.
//
// Below is a diagram illustrating the setup.
//
//  [Default network namespace] +------------------+-------------+
//                              |                  |             |
//       +--------+      +--------------+  +--------------+      |
//       |  Tast  |----->|  p2p_server  |  |  p2p_client  |      |
//       +--------+ HTTP +--------------+  +--------------+      |
//            |                                                  | D-Bus
//       +------------------------------------------------+      |  IPC
//       |         veth-default (169.254.100.1)           |      |
//       +------------------------------------------------+      |
//                              | mDNS/DNS-SD                    |
//  ----------------------------+-----------------------------------------
//                              |                                |
//       +------------------------------------------------+      |
//       |         veth-isolated (169.254.100.2)          |      |
//       +------------------------------------------------+      |
//                              |                                |
//                      +----------------+                       |
//                      |  avahi-daemon  |<----------------------+
//                      +----------------+
//
//  [Isolated network namespace "tastns"]
//
// Note that it would be more preferable to run the Tast test process, instead
// of avahi-daemon, in an isolated network namespace, because it is more similar
// to the real configurations (e.g. iptables rules apply to avahi-daemon). But
// it is difficult to make the Tast test process enter a network namespace since
// setns(2) works per-thread but Go programs do not have a way to call system
// calls on all threads at once. See the following issues for details:
//
//  https://github.com/vishvananda/netns/issues/17
//  https://github.com/golang/go/issues/1435
//
package p2p

import (
	"context"
	"os"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	// ServiceType is the type of the P2P service (in terms of DNS-SD).
	ServiceType = "_cros_p2p._tcp"

	// ServicePort is the TCP port number where p2p-http-server serves HTTP
	// requests by default.
	ServicePort = 16725

	// SharedDir is the directory to hold the files served by p2p-http-server.
	SharedDir = "/var/cache/p2p"

	// NSName is the name of an isolated network namespace P2P tests create to
	// run avahi-daemon in.
	NSName = "tastns"

	// DefaultNSIP is the primary IPv4 address of the virtual network interface in
	// the default network namespace where Tast test process runs.
	DefaultNSIP = "169.254.100.1"

	// IsolatedNSIP is the IPv4 address of the virtual network interface in the
	// "tastns" network namespace where avahi-daemon runs.
	IsolatedNSIP = "169.254.100.2"

	defaultIFName  = "veth-default"
	isolatedIFName = "veth-isolated"
)

// SetUp does some setup for P2P tests.
func SetUp(ctx context.Context) error {
	if err := upstart.StopJob(ctx, "p2p"); err != nil {
		return errors.Wrap(err, "failed to stop p2p")
	}

	if err := clearSharedDir(); err != nil {
		return errors.Wrap(err, "failed to clear the p2p shared directory")
	}

	if err := createVirtualNetwork(ctx); err != nil {
		return errors.Wrap(err, "failed to create the virtual network")
	}

	// Restart avahi in the network namespace and wait to be ready.
	if err := upstart.RestartJob(ctx, "avahi", upstart.WithArg("NETNS", NSName)); err != nil {
		return errors.Wrap(err, "failed to restart avahi")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "p2p-client", "--num-connections").Run()
	}, nil); err != nil {
		return errors.Wrap(err, "failed to wait avahi startup")
	}
	return nil
}

// CleanUp does some cleanup for P2P tests.
func CleanUp(ctx context.Context) error {
	// Restart avahi in the default network namespace.
	if err := upstart.RestartJob(ctx, "avahi"); err != nil {
		return errors.Wrap(err, "failed to restart avahi")
	}

	if err := destroyVirtualNetwork(ctx, failOnErrors); err != nil {
		return errors.Wrap(err, "failed to destroy the virtual network")
	}
	return nil
}

func clearSharedDir() error {
	if err := os.RemoveAll(SharedDir); err != nil {
		return err
	}
	if err := os.Mkdir(SharedDir, 0755); err != nil {
		return err
	}
	return os.Chmod(SharedDir, 0755)
}

func createVirtualNetwork(ctx context.Context) error {
	destroyVirtualNetwork(ctx, ignoreErrors)

	const multicastSubnet = "224.0.0.0/4"

	for _, args := range [][]string{
		// Create an isolated network namespace.
		{"netns", "add", NSName},
		// Create a virtual network interface pair, and put one of them to the isolated
		// network namespace.
		{"link", "add", defaultIFName, "type", "veth", "peer", "name", isolatedIFName},
		{"link", "set", isolatedIFName, "netns", NSName},
		// Set up the network interface in the default network namespace.
		{"addr", "add", DefaultNSIP + "/24", "dev", defaultIFName},
		{"link", "set", defaultIFName, "up"},
		{"route", "add", multicastSubnet, "dev", defaultIFName},
		// Set up the network interface in the isolated network namespace.
		{"netns", "exec", NSName, "ip", "addr", "add", IsolatedNSIP + "/24", "dev", isolatedIFName},
		{"netns", "exec", NSName, "ip", "link", "set", isolatedIFName, "up"},
		{"netns", "exec", NSName, "ip", "route", "add", multicastSubnet, "dev", isolatedIFName},
	} {
		cmd := testexec.CommandContext(ctx, "ip", args...)
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrapf(err, "ip %s failed", shutil.EscapeSlice(args))
		}
	}
	return nil
}

type errorMode int

const (
	failOnErrors errorMode = iota
	ignoreErrors
)

func destroyVirtualNetwork(ctx context.Context, mode errorMode) error {
	for _, args := range [][]string{
		// Delete the network interface in the default network namespace. This automatically
		// deletes the peer in the isolated network namespace.
		{"link", "del", defaultIFName},
		// Delete the isolated network namespace.
		{"netns", "del", NSName},
	} {
		cmd := testexec.CommandContext(ctx, "ip", args...)
		if err := cmd.Run(); err != nil && mode == failOnErrors {
			cmd.DumpLog(ctx)
			return errors.Wrapf(err, "ip %s failed", shutil.EscapeSlice(args))
		}
	}
	return nil
}

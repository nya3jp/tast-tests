// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"
	// "time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/network/allowlist"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	// TODO(acostinas, b/191845062) Re-enable the test when OTA credentials are available in tast tests.
	// testing.AddTest(&testing.Test{
	// 	Func: SystemServicesConnectivity,
	// 	Desc: "Test that system services work behind a firewall configured according to our support page",
	// 	Contacts: []string{
	// 		"acostinas@google.com", // Test author
	// 		"chromeos-commercial-networking@google.com",
	// 	},
	// 	Attr:         []string{"group:mainline", "informational"},
	// 	Data:         []string{"allowlist_ssl_inspection.json"},
	// 	SoftwareDeps: []string{"reboot", "chrome", "chrome_internal"},
	// 	ServiceDeps:  []string{"tast.cros.network.AllowlistService", "tast.cros.network.ProxyService"},
	// 	VarDeps: []string{
	// 		"allowlist.username",
	// 		"allowlist.password",
	// 	},
	// 	Timeout: 12 * time.Minute,
	// })
}

// SystemServicesConnectivity calls the AllowlistService to setup a firewall and verifies system services connectivity.
func SystemServicesConnectivity(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		// Since this test is changing the iptable rules to create a firewall on the DUT, we need to reboot to make sure the
		// DUT gets back to its initial state, which doesn't restrict connectivity to http/s default ports.
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	a, err := allowlist.ReadHostnames(ctx, s.DataPath("allowlist_ssl_inspection.json"), false, false)
	if err != nil {
		s.Fatal("Failed to read hostnames: ", err)
	}

	const port uint32 = 3129

	// Start an HTTP proxy instance on the DUT which only allows connections to the allowlisted hostnames.
	proxyClient := network.NewProxyServiceClient(cl.Conn)
	response, err := proxyClient.StartServer(ctx,
		&network.StartServerRequest{
			Port:      port,
			Allowlist: a,
		})
	if err != nil {
		s.Fatal("Failed to start a local proxy on the DUT: ", err)
	}

	al := network.NewAllowlistServiceClient(cl.Conn)
	if _, err := al.SetupFirewall(ctx, &network.SetupFirewallRequest{AllowedPort: port}); err != nil {
		s.Fatal("Failed to setup a firewall on the DUT: ", err)
	}

	user := s.RequiredVar("allowlist.username")
	password := s.RequiredVar("allowlist.password")
	if _, err := al.GaiaLogin(ctx, &network.GaiaLoginRequest{
		Username: user, Password: password, ProxyHostAndPort: response.HostAndPort}); err != nil {
		s.Fatal("Failed to login through the proxy: ", err)
	}

	if err = runTLSDate2(ctx, s.DUT().Conn()); err != nil {
		s.Fatal("Failed to run tlsdate with system-proxy: ", err)
	}
	// TODO(acostinas): Find a way to test update_engine and crash_sender behind the firewall.
}

// runTLSDate2 runs tlsdate once, in the foreground. Returns an error if tlsdate didn't use system-proxy to connect to the remote host
// or if the certificate verification failed.
// TODO(acostinas): Consider merging with runTLSDate.
func runTLSDate2(ctx context.Context, conn *ssh.Conn) error {
	// tlsdated is a CrOS daemon that runs the tlsdate binary periodically in the background and does proxy resolution through Chrome.
	// Prepend `timeout` to the command that runs tlsdated to send SIGTERM to the daemon after the first invocation (otherwise tlsdated
	// will run in the foreground until the connection times out).
	// The `-m <n>` option means tlsdate should run at most once every n seconds in steady state
	// The `-p` option means dry run.
	// TODO(acostinas,b/179762130) Remove timeout once tlsdated has an option to exit after the first invocation.
	out, err := conn.Command("timeout", "20", "/usr/bin/tlsdated", "-p", "-m", "60", "--", "/usr/bin/tlsdate", "-v", "-C", "/usr/share/chromeos-ca-certificates", "-l").CombinedOutput(ctx)

	//  The exit code 124 indicates that timeout sent a SIGTERM to terminate tlsdate.
	if err != nil && !strings.Contains(err.Error(), "Process exited with status 124") {
		return errors.Wrap(err, "error running tlsdate")
	}
	var result = string(out)
	const successMsg = "V: certificate verification passed"
	if !strings.Contains(result, successMsg) {
		return errors.Errorf("failed to verify the certificate: %s", result)
	}

	return nil
}

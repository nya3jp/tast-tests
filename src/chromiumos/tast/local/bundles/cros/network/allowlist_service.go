// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/proxy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

const (
	iptablesCmd  = "/sbin/iptables"
	ip6tablesCmd = "/sbin/ip6tables"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterAllowlistServiceServer(srv, &AllowlistService{s: s})
		},
	})
}

// AllowlistService implements the tast.cros.network.AllowlistService gRPC service.
type AllowlistService struct {
	s *testing.ServiceState

	cr    *chrome.Chrome
	proxy *proxy.Server
}

func (a *AllowlistService) SetupFirewall(ctx context.Context, req *network.SetupFirewallRequest) (*empty.Empty, error) {
	// Start a proxy server which only allows connections to hostnames specified in req.Hostnames.
	a.proxy = proxy.NewServer()
	if err := a.proxy.Start(ctx, 3128, nil, req.Hostnames); err != nil {
		return nil, errors.Wrap(err, "failed to setup proxy server")
	}

	// Allow traffic from the proxy through the firewall.
	cmds := []string{iptablesCmd, ip6tablesCmd}
	args := []string{"-A", "OUTPUT", "-p", "tcp", "-m", "tcp", "--sport", "3128", "-j", "ACCEPT"}
	if err := executeIptables(ctx, cmds, args); err != nil {
		return nil, err
	}

	// Drop http and https traffic.
	protocols := []string{"tcp", "udp"}
	ports := []string{"80", "443"}

	for _, pr := range protocols {
		for _, po := range ports {
			args := []string{"-A", "OUTPUT", "-p", pr, "--dport", po, "-j", "REJECT"}
			if err := executeIptables(ctx, cmds, args); err != nil {
				return nil, err
			}
		}
	}

	// Move this rule down so that the rules above have priority.
	args = []string{"-D", "OUTPUT", "-m", "state", "--state", "NEW,RELATED,ESTABLISHED", "-j", "ACCEPT"}
	if err := executeIptables(ctx, cmds, args); err != nil {
		return nil, err
	}
	args = []string{"-A", "OUTPUT", "-m", "state", "--state", "NEW,RELATED,ESTABLISHED", "-j", "ACCEPT"}
	if err := executeIptables(ctx, cmds, args); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

func (a *AllowlistService) GaiaLogin(ctx context.Context, req *network.GaiaLoginRequest) (*empty.Empty, error) {
	// Chrome uses the proxy address of the proxy instance started by calling `SetupFirewall` as a start up argument.
	if a.proxy == nil {
		return nil, errors.New("Please setup a firewall before logging in by calling SetupFirewall()")
	}

	cr, err := chrome.New(
		ctx,
		chrome.Auth(req.Username, req.Password, ""),
		chrome.GAIALogin(),
		chrome.ProdPolicy(),
		chrome.ARCSupported(),
		chrome.ExtraArgs("--proxy-server=http://"+a.proxy.HostAndPort))
	if err != nil {
		return nil, err
	}
	a.cr = cr
	return &empty.Empty{}, nil
}

func (a *AllowlistService) TestSystemServicesConnectivity(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if a.cr == nil {
		return nil, errors.New("Please start a new Chrome instance that uses the firewall by calling GaiaLogin()")
	}

	if err := testTlsdateConnectivity(ctx); err != nil {
		return nil, err
	}
	// TODO(acostinas): Find a way to test update_engine and crash_sender behind the firewall.
	return &empty.Empty{}, nil
}

func (a *AllowlistService) TestArcConnectivity(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return nil, errors.New("method TestArcConnectivity not implemented")

}

func (a *AllowlistService) TestExtensionConnectivity(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return nil, errors.New("method TestExtensionConnectivity not implemented")
}

func executeIptables(ctx context.Context, cmds, args []string) error {
	for _, cmd := range cmds {
		if err := testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to add iptables rule: %v", args)
		}
	}
	return nil
}

func testTlsdateConnectivity(ctx context.Context) error {
	// tlsdated is a CrOS daemon that runs the tlsdate binary periodically in the background and does proxy resolution through Chrome.
	// Prepend `timeout` to the command that runs tlsdated to send SIGTERM to the daemon after the first invocation (otherwise tlsdated
	// will run in the foreground until the connection times out).
	// The `-m <n>` option means tlsdate should run at most once every n seconds in steady state
	// The `-p` option means dry run.
	// TODO(acostinas,b/179762130) Remove timeout once tlsdated has an option to exit after the first invocation.
	out, err := testexec.CommandContext(ctx, "timeout", "20", "/usr/bin/tlsdated", "-p", "-m", "60", "--", "/usr/bin/tlsdate", "-v", "-C", "/usr/share/chromeos-ca-certificates", "-l").CombinedOutput()

	//  The exit code 124 indicates that timeout sent a SIGTERM to terminate tlsdate.
	if err != nil && !strings.Contains(err.Error(), "exit status 124") {
		return errors.Wrap(err, "error running tlsdate")
	}
	var result = string(out)

	const successMsg = "V: certificate verification passed"
	if !strings.Contains(result, successMsg) {
		return errors.Errorf("certificate verification failed: %s", result)
	}
	return nil
}

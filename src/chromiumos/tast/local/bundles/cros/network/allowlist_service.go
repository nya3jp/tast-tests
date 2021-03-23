// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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

	cr *chrome.Chrome
}

func (a *AllowlistService) SetupFirewall(ctx context.Context, req *network.SetupFirewallRequest) (*empty.Empty, error) {
	// Allow traffic from the proxy through the firewall.
	cmds := []string{iptablesCmd, ip6tablesCmd}
	args := []string{"-I", "OUTPUT", "-p", "tcp", "-m", "tcp", "--sport", fmt.Sprint(req.AllowedPort), "-j", "ACCEPT"}
	if err := executeIptables(ctx, cmds, args); err != nil {
		return nil, err
	}
	// Allow connection from the proxy.
	args = []string{"-I", "FORWARD", "-p", "tcp", "-i", "arc_ns+", "-j", "ACCEPT"}
	if err := executeIptables(ctx, cmds, args); err != nil {
		return nil, err
	}
	// Drop http and https traffic.
	protocols := []string{"tcp", "udp"}
	ports := []string{"80", "443"}

	for _, pr := range protocols {
		for _, po := range ports {
			// Add this rule with rule-number 2 so that the first rule above, which allows proxy traffic for the OUTPUT chain, has priority.
			args := []string{"-I", "OUTPUT", "2", "-p", pr, "--dport", po, "-j", "REJECT"}
			if err := executeIptables(ctx, cmds, args); err != nil {
				return nil, err
			}
			// Add this rule with rule-number 2 so that the second rule above, which allows proxy traffic for the FORWARD chain, has priority.
			args = []string{"-I", "FORWARD", "2", "-p", pr, "--dport", po, "-j", "REJECT"}
			if err := executeIptables(ctx, []string{iptablesCmd}, args); err != nil {
				return nil, err
			}
		}
	}
	return &empty.Empty{}, nil
}

func (a *AllowlistService) GaiaLogin(ctx context.Context, req *network.GaiaLoginRequest) (*empty.Empty, error) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password, GAIAID: ""}),
		chrome.ProdPolicy(),
		chrome.ARCSupported(),
		chrome.ExtraArgs("--proxy-server=http://"+req.ProxyHostAndPort))
	if err != nil {
		return nil, err
	}
	a.cr = cr
	return &empty.Empty{}, nil
}

// CheckArcAppInstalled verifies that ARC provisioning and installing ARC apps by checking that force installed apps are successfully installed.
func (a *AllowlistService) CheckArcAppInstalled(ctx context.Context, req *network.CheckArcAppInstalledRequest) (*empty.Empty, error) {
	if a.cr == nil {
		return nil, errors.New("Please start a new Chrome instance that uses the firewall by calling GaiaLogin()")
	}

	td, _ := testing.ContextOutDir(ctx)
	arc, err := arc.New(ctx, td)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	defer arc.Close(ctx)

	// Ensure that Android packages are force-installed by ARC policy.
	ctx, st := timing.Start(ctx, "wait_packages")
	defer st.End()

	testing.ContextLog(ctx, "Waiting for packages to be installed")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		pkgs, err := arc.InstalledPackages(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}

		if _, appFound := pkgs[req.AppName]; appFound {
			return nil
		}
		return errors.New("failed to install 3rd party app")
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 180 * time.Second}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// CheckExtensionInstalled verifies that specified extension is installed by performing a full-text search on the chrome://extensions page.
func (a *AllowlistService) CheckExtensionInstalled(ctx context.Context, req *network.CheckExtensionInstalledRequest) (*empty.Empty, error) {
	// Connect to Test API to use it with the UI library.
	tconn, err := a.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	const extensionURL = "chrome://extensions"

	sconn, err := a.cr.NewConn(ctx, extensionURL)
	if err != nil {
		return nil, err
	}
	defer sconn.Close()

	desc := ui.FindParams{
		Name: req.ExtensionTitle,
		Role: ui.RoleTypeStaticText,
	}

	node, err := ui.FindWithTimeout(ctx, tconn, desc, 3*time.Minute)
	if err != nil {
		return nil, err
	}
	defer node.Release(ctx)
	return &empty.Empty{}, nil
}

func executeIptables(ctx context.Context, cmds, args []string) error {
	for _, cmd := range cmds {
		if err := testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to add iptables rule: %s %v", cmd, args)
		}
	}
	return nil
}

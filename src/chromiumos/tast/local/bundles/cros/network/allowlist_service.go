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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/firewall"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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
	params := firewall.CreateFirewallParams{
		AllowPorts:      []string{fmt.Sprint(req.AllowedPort)},
		AllowInterfaces: []string{"arc_ns+"},
		AllowProtocols:  []string{"tcp"},
		// Drop http and https traffic.
		BlockPorts:     []string{"80", "443"},
		BlockProtocols: []string{"tcp", "udp"},
	}
	if err := firewall.CreateFirewall(ctx, params); err != nil {
		return nil, err
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

	desc := nodewith.Name(req.ExtensionTitle).Role(role.StaticText)
	ui := uiauto.New(tconn).WithTimeout(3 * time.Minute)

	if err := ui.WaitUntilExists(desc)(ctx); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

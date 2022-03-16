// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package osinstall

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/osinstall"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OsInstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test OS install (the DUT must be started back up again after install succeeds)",
		Contacts: []string{
			"chromeos-flex-eng@google.com",
			"nicholasbishop@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.osinstall.OsInstallService"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		// Allow up to 20 minutes for install, plus some extra time for the DUT
		// to be started back up.
		Timeout: 25 * time.Minute,
	})
}

func runOsInstallAndShutdown(ctx context.Context, s *testing.State) *osinstall.GetOsInfoResponse {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := osinstall.NewOsInstallServiceClient(cl.Conn)

	preInstallInfo, err := client.GetOsInfo(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get pre-install OS info: ", err)
	}
	s.Log("Pre-install OS info: ", preInstallInfo)

	// Check that the DUT is running from an installer.
	if !preInstallInfo.IsRunningFromInstaller {
		s.Fatal("The OS is not running from an installer")
	}

	// Launch Chrome OOBE.
	req := osinstall.StartChromeRequest{
		SigninProfileTestExtensionID: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
	}
	s.Log("Starting Chrome")
	if _, err := client.StartChrome(ctx, &req); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Start the installation process.
	s.Log("Running OS install and waiting for it to complete")
	if _, err := client.RunOsInstall(ctx, &empty.Empty{}); err != nil {
		s.Fatal("OS install failed: ", err)
	}

	// Power off.
	if _, err := client.ShutDown(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to shut down: ", err)
	}

	return preInstallInfo
}

func OsInstall(ctx context.Context, s *testing.State) {
	preInstallInfo := runOsInstallAndShutdown(ctx, s)

	// Wait for the DUT to shut down, then wait for it to come back up
	// (hopefully with the newly installed system).
	s.Log("Waiting for the DUT to shut down and then become reachable again")
	s.DUT().WaitUnreachable(ctx)
	s.DUT().WaitConnect(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Get the system info to verify that the install succeeded.
	client := osinstall.NewOsInstallServiceClient(cl.Conn)
	info, err := client.GetOsInfo(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get OS info: ", err)
	}

	if info.IsRunningFromInstaller {
		s.Fatal("Still running from an installer")
	}

	if preInstallInfo.Version != info.Version {
		s.Fatalf("Installed OS version does not match installer version: %s != %s", preInstallInfo.Version, info.Version)
	}
}

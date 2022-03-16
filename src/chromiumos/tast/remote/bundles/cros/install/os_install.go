// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package install

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/install"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OsInstall,
		Desc: "TODO",
		Contacts: []string{
			"nicholasbishop@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.install.OsInstallService"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func RunOsInstallAndShutdown(ctx context.Context, s *testing.State) *install.GetOsInfoResponse {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := install.NewOsInstallServiceClient(cl.Conn)

	pre_install_info, err := client.GetOsInfo(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get pre-install OS info: ", err)
	}
	s.Log("Pre-install OS info: ", pre_install_info)

	// Check that the DUT is running from an installer.
	if !pre_install_info.IsRunningFromInstaller {
		s.Fatal("The OS is not running from an installer")
	}

	// Start the installation process. The DUT will shut down at the end.
	req := install.RunOsInstallRequest{
		SigninProfileTestExtensionID: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
	}
	if _, err := client.RunOsInstall(ctx, &req); err != nil {
		s.Fatal("OS install failed: ", err)
	}

	return pre_install_info
}

// Create a fresh connection to the DUT and get OS info.
func GetFreshOsInfo(ctx context.Context, s *testing.State) (*install.GetOsInfoResponse, error) {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		return nil, err
	}
	defer cl.Close(ctx)

	client := install.NewOsInstallServiceClient(cl.Conn)
	info, err := client.GetOsInfo(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	return info, nil
}

func OsInstall(ctx context.Context, s *testing.State) {
	pre_install_info := RunOsInstallAndShutdown(ctx, s)

	s.DUT().WaitUnreachable(ctx)

	s.DUT().WaitConnect(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := install.NewOsInstallServiceClient(cl.Conn)
	info, err := client.GetOsInfo(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get OS info: ", err)
	}

	if info.IsRunningFromInstaller {
		s.Fatal("Still running from an installer")
	}

	if pre_install_info.Version != info.Version {
		s.Fatal("Installed OS version does not match installer version: %s != %s", pre_install_info.Version, info.Version)
	}
}

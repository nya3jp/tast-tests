// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	factoryservice "chromiumos/tast/services/cros/factory"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	// EnsureToolkit is the fixture which installs factory toolkit at set up
	// with a reboot, and uninstall the toolkit at tear down with a reboot.
	// It is required to add `SoftwareDeps` from `EnsureToolkitSoftwareDeps`
	// for tests using this fixture.
	EnsureToolkit = "ensureToolkit"

	// reboot takes 30 seconds to pass the boot option selection in developer
	// mode, plus enough time as buffer to wait for the system and networking to be ready
	rebootTimeout = 3 * time.Minute
)

// EnsureToolkitSoftwareDeps is the SoftwareDeps required by `EnsureToolkit`
var EnsureToolkitSoftwareDeps = []string{"reboot", "factory_flow"}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            EnsureToolkit,
		Desc:            "Fixture for ensuring toolkit is installed before test and uninstalled after test",
		Contacts:        []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Impl:            &ensureToolkitFixt{},
		SetUpTimeout:    rebootTimeout + time.Minute, // reboot and do toolkit installation
		TearDownTimeout: rebootTimeout + time.Minute, // reboot and do toolkit uninstallation
		ServiceDeps:     []string{"tast.cros.factory.Toolkit"},
	})
}

type ensureToolkitFixt struct{}

func (*ensureToolkitFixt) Reset(ctx context.Context) error                        { return nil }
func (*ensureToolkitFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*ensureToolkitFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (e *ensureToolkitFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// TODO(b/204845193): workaround as fixture do not recover connection
	// if failed from previous test.
	s.DUT().Connect(ctx)

	ver, err := installFactoryToolKit(ctx, s)
	if err != nil {
		s.Fatal("Install fail: ", err)
	}
	s.Logf("Installed factory toolkit with version: %s", ver)
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Reboot device fail: ", err)
	}
	s.Log("Device rebooted successfully")
	return nil
}

func (e *ensureToolkitFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	dut := s.DUT()
	const toolkitEnabledPath = "/usr/local/factory/enabled"
	removeEnabledCmd := dut.Conn().CommandContext(ctx, "rm", "-rf", toolkitEnabledPath)
	if err := removeEnabledCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Disable toolkit fail: ", err)
	}
	if err := dut.Reboot(ctx); err != nil {
		s.Fatal("Reboot device fail: ", err)
	}
	if err := uninstallFactoryToolKit(ctx, s); err != nil {
		s.Fatal("Failed to uninstall factory toolkit: ", err)
	}
	s.Log("Toolkit uninstalled successfully")
}

func installFactoryToolKit(ctx context.Context, s *testing.FixtState) (string, error) {
	client, connClient, err := getFactoryServiceClient(ctx, s)
	if err != nil {
		return "", err
	}
	defer connClient.Close(ctx)

	req := factoryservice.InstallRequest{NoEnable: false}
	res, err := client.Install(ctx, &req)
	if err != nil {
		return "", err
	}
	return res.Version, nil
}

func uninstallFactoryToolKit(ctx context.Context, s *testing.FixtState) error {
	client, connClient, err := getFactoryServiceClient(ctx, s)
	if err != nil {
		return err
	}
	defer connClient.Close(ctx)

	_, err = client.Uninstall(ctx, &empty.Empty{})
	return err
}

func getFactoryServiceClient(ctx context.Context, s *testing.FixtState) (factoryservice.ToolkitClient, *rpc.Client, error) {
	connClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	client := factoryservice.NewToolkitClient(connClient.Conn)
	return client, connClient, nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	toolkitInstallerName = "install_factory_toolkit.run"
	// the path should be synced with factory-mini.ebuild
	toolkitInstallerPath = "/usr/local/factory-toolkit/" + toolkitInstallerName
	// Get the factory toolkit version.
	toolkitVersionPath = "/usr/local/factory/TOOLKIT_VERSION"
	// reboot takes 30 seconds to pass the boot option selection in developer mood, plus 30 seconds as buffer to wait for the system to be ready
	rebootTimeout = time.Minute
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "ensureToolkit",
		Desc:            "Fixture for ensuring toolkit is installed before test and uninstalled after test",
		Contacts:        []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Impl:            &ensureToolkitFixt{},
		SetUpTimeout:    rebootTimeout,
		TearDownTimeout: rebootTimeout,
	})
}

type ensureToolkitFixt struct{}

func (*ensureToolkitFixt) Reset(ctx context.Context) error                        { return nil }
func (*ensureToolkitFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*ensureToolkitFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (e *ensureToolkitFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ver, err := installFactoryToolKit(ctx, s.DUT().Conn())
	if err != nil {
		s.Fatal("Install fail: ", err)
	}
	s.Logf("Installed factory toolkit with version: %s", ver)
	if err := rebootDeviceToReady(ctx, s.DUT()); err != nil {
		s.Fatal("Reboot device fail: ", err)
	}
	s.Log("Device rebooted successfully")
	return nil
}

func (e *ensureToolkitFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	dut := s.DUT()
	uninstallCmd := dut.Conn().CommandContext(ctx, "factory_uninstall", "--yes")
	if err := uninstallCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to uninstall factory toolkit: ", err)
	}
	if err := rebootDeviceToReady(ctx, dut); err != nil {
		s.Fatal("Reboot device fail: ", err)
	}
}

func installFactoryToolKit(ctx context.Context, conn *ssh.Conn) (version string, err error) {
	checkInstallerExistenceCmd := conn.CommandContext(ctx, "ls", toolkitInstallerPath)
	if err := checkInstallerExistenceCmd.Run(ssh.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "failed to find factory toolkit installer: %s", toolkitInstallerPath)
	}

	// TODO(b/150189948): Support installing toolkit from artifacts
	// factory_image.zip
	return installFactoryToolKitFromToolkitInstaller(ctx, toolkitInstallerPath, conn)
}

func installFactoryToolKitFromToolkitInstaller(ctx context.Context, installerPath string, conn *ssh.Conn) (version string, err error) {
	installCmd := conn.CommandContext(ctx, installerPath, "--", "--yes")
	if err := installCmd.Run(ssh.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to install factory toolkit")
	}

	versionByte, err := linuxssh.ReadFile(ctx, conn, toolkitVersionPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read version file")
	}
	version = strings.TrimSpace(string(versionByte))
	return version, nil
}

func rebootDeviceToReady(ctx context.Context, dut *dut.DUT) error {
	err := dut.Reboot(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to reboot device")
	}
	err = dut.WaitConnect(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to reconnect to device")
	}
	return nil
}

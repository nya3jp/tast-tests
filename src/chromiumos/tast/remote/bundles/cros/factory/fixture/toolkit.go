// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// EnsureToolkit is the fixture installs factory toolkit at set up with
	// a reboot, uninstall the toolkit at tear down with a reboot. It is
	// required to add `SoftwareDeps` from `EnsureToolkitSoftwareDeps` for
	// tests using this fixture.
	EnsureToolkit = "ensureToolkit"

	toolkitInstallerName = "install_factory_toolkit.run"
	// the path should be synced with factory-mini.ebuild
	toolkitInstallerPath = "/usr/local/factory-toolkit/" + toolkitInstallerName
	// Get the factory toolkit version.
	toolkitVersionPath = "/usr/local/factory/TOOLKIT_VERSION"
	// the existence of enabled file determines whether DUT booted with running the toolkit
	toolkitEnabledPath = "/usr/local/factory/enabled"
	// reboot takes 30 seconds to pass the boot option selection in developer
	// mode, plus enough time as buffer to wait for the system and networking to be ready
	rebootTimeout = 3 * time.Minute
	testListName  = "generic_tast"
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
	})
}

type ensureToolkitFixt struct{}

func (*ensureToolkitFixt) Reset(ctx context.Context) error                        { return nil }
func (*ensureToolkitFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*ensureToolkitFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (e *ensureToolkitFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// FIXME(b/204845193): workaround as fixture do not recover connection
	// if failed from previous test
	s.DUT().Connect(ctx)

	ver, err := installFactoryToolKit(ctx, s.DUT().Conn())
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
	removeEnabledCmd := dut.Conn().CommandContext(ctx, "rm", "-rf", toolkitEnabledPath)
	if err := removeEnabledCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Disable toolkit fail: ", err)
	}
	if err := dut.Reboot(ctx); err != nil {
		s.Fatal("Reboot device fail: ", err)
	}
	uninstallCmd := dut.Conn().CommandContext(ctx, "factory_uninstall", "--yes")
	if err := uninstallCmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to uninstall factory toolkit: ", err)
	}
	s.Log("Toolkit uninstalled successfully")
}

func installFactoryToolKit(ctx context.Context, conn *ssh.Conn) (version string, err error) {
	checkInstallerExistenceCmd := conn.CommandContext(ctx, "ls", toolkitInstallerPath)
	if err := checkInstallerExistenceCmd.Run(ssh.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "failed to find factory toolkit installer: %s", toolkitInstallerPath)
	}

	// TODO(b/150189948): Support installing toolkit from artifacts
	// factory_image.zip
	if version, err = installFactoryToolKitFromToolkitInstaller(ctx, toolkitInstallerPath, conn); err != nil {
		return "", errors.Wrapf(err, "can not install toolkit: %s", toolkitInstallerPath)
	}

	// set tast specific test list to prevent from unexpected behavior, such
	// as cr50 update and reboot, etc.
	setTestListCmd := conn.CommandContext(ctx, "factory", "test-list", testListName)
	if err := setTestListCmd.Run(ssh.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "can not set test list to %s: %s", testListName, toolkitInstallerPath)
	}

	return version, nil

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

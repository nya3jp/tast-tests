// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

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
	toolkitName          = "factory_toolkit"
	toolkitInstallerName = "install_factory_toolkit.run"
	// the path should be synced with factory-mini.ebuild
	toolkitInstallerPath = "/usr/local/factory-toolkit/" + toolkitInstallerName
	// Get the factory toolkit version.
	toolkitVersionPath         = "/usr/local/factory/TOOLKIT_VERSION"
	toolkitPreconditionTimeout = 2 * time.Minute
)

type toolkitPre struct {
	// ensured records whether toolkit is installed and rebooted in this Precondition.
	ensured bool
}

var toolkitEnsurer = new(toolkitPre)

// GetToolkitEnsurer returns toolkit ensurer as Precondition to make sure toolkit is installed before test and uninstalled after all tests passed.
func GetToolkitEnsurer() *toolkitPre {
	return toolkitEnsurer
}

func (t *toolkitPre) String() string                                 { return toolkitName }
func (t *toolkitPre) Timeout() time.Duration                         { return toolkitPreconditionTimeout }
func (t *toolkitPre) Close(ctx context.Context, s *testing.PreState) {}

func (t *toolkitPre) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if !t.ensured {
		ver, err := installFactoryToolKit(ctx, s, s.DUT().Conn())
		if err != nil {
			s.Fatal("Install fail: ", err)
		}
		s.Logf("Installed factory toolkit with version: %s", ver)
		if err := rebootDeviceToReady(ctx, s.DUT()); err != nil {
			s.Fatal("Reboot device fail: ", err)
		}
		s.Log("Device rebooted successfully")
		t.ensured = true
	}
	return nil
}

func installFactoryToolKit(ctx context.Context, s *testing.PreState, conn *ssh.Conn) (version string, err error) {
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

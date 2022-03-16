// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package toolkit

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	// ToolkitInstallerPath is the full path of the installer embedded in
	// the test image. The path should be synced with factory-mini.ebuild.
	ToolkitInstallerPath = "/usr/local/factory-toolkit/install_factory_toolkit.run"

	testListName    = "generic_tast"
	factoryRootPath = "/usr/local/factory"
)

// Installer contains configurations to run the installation of factory toolkit.
// Currently, we only support to install toolkit from a installer.
type Installer struct {
	// The path of factory toolkit installer to be used during installation.
	InstallerPath string
	// TODO(b/150189948) The image path  to retrieve a installer, currently
	// not supported.
	ImagePath string
	// NoEnable configures to install toolkit without starting the factory
	// mode after reboot. It comes from the installation option --no-enable
	// in the factory toolkit installer.
	NoEnable bool
}

// InstallFactoryToolKit installs the factory toolkit. Currently installs with
// the installer specified in the `InstallerPath`.
func (i *Installer) InstallFactoryToolKit(ctx context.Context) (string, error) {
	return i.installFactoryToolKitFromToolkitInstaller(ctx, i.InstallerPath)
}

// installFactoryToolKitFromToolkitInstaller installs factory toolkit with the
// installer and returns the version of the installed toolkit. The existence of
// the installer is first checked, installed, then probe the version file as
// the return value. Also configure toolkit to fit the environment in the lab.
func (i *Installer) installFactoryToolKitFromToolkitInstaller(ctx context.Context, installerPath string) (version string, err error) {
	if _, err := os.Stat(installerPath); err != nil {
		return "", errors.Wrapf(err, "failed to find factory toolkit installer: %s", installerPath)
	}

	installArgs := []string{"--", "--yes"}
	if i.NoEnable {
		installArgs = append(installArgs, "--no-enable")
	}
	installCmd := testexec.CommandContext(ctx, installerPath, installArgs...)
	if err := installCmd.Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to install factory toolkit")
	}

	// Get the factory toolkit version.
	toolkitVersionPath := filepath.Join(factoryRootPath, "/TOOLKIT_VERSION")
	b, err := ioutil.ReadFile(toolkitVersionPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read version file")
	}
	version = strings.TrimSpace(string(b))
	err = i.configureToolkitWithLabEnvironment(ctx)
	return version, err
}

// installFactoryToolKitFromFactoryImage retrieves installer from the zip image,
// then executes the installation.
func (i *Installer) installFactoryToolKitFromFactoryImage(ctx context.Context) (version string, err error) {
	// Create an temp directory.
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	const installerPath = "toolkit/install_factory_toolkit.run"
	extractedInstallerPath := filepath.Join(tempDir, filepath.Base(installerPath))
	if err := unzipFile(extractedInstallerPath, i.ImagePath, installerPath); err != nil {
		return "", errors.Wrap(err, "failed to unzip toolkit installer")
	}

	return i.installFactoryToolKitFromToolkitInstaller(ctx, extractedInstallerPath)
}

// configureToolkitWithLabEnvironment sets up configurations for factory toolkit
// so that it can run with tast and does not break other tests due to side
// effects of the toolkit itself.
func (i *Installer) configureToolkitWithLabEnvironment(ctx context.Context) error {
	// set tast specific test list to prevent from unexpected behavior, such
	// as cr50 update and reboot, etc.
	setTestListCmd := testexec.CommandContext(ctx, "factory", "test-list", testListName)
	if err := setTestListCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute `factory test-list` command")
	}
	// Verify the config content is correct.
	type DataFile struct {
		ID string `json:"id"`
	}
	var data DataFile
	testListConfigPath := filepath.Join(factoryRootPath, "py/test/test_lists/active_test_list.json")
	b, err := ioutil.ReadFile(testListConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to read TestList config file")
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return errors.Wrap(err, "failed to decode JSON")
	}
	if data.ID != testListName {
		return errors.Wrap(err, "testList config file doesn't contain the correct test id")
	}

	// TODO(b/205779346): workaround to prevent disk_space_hacks.sh from
	// deleting directories needed by other test.
	diskSpaceHackScriptPath := filepath.Join(factoryRootPath, "/init/init.d/disk_space_hacks.sh")
	if err := os.Remove(diskSpaceHackScriptPath); err != nil {
		return errors.Wrapf(err, "cannot remove %s", diskSpaceHackScriptPath)
	}
	return nil
}

// UninstallFactoryToolKit uninstall the toolkit via command factory_uninstall
// with --yes to prevent hanging for the user response.
func UninstallFactoryToolKit(ctx context.Context) error {
	uninstallCmd := testexec.CommandContext(ctx, "factory_uninstall", "--yes")
	return uninstallCmd.Run(testexec.DumpLogOnError)
}

func unzipFile(dstPath, zipFilePath, srcPathInZip string) error {
	archive, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	targetFile, err := archive.Open(srcPathInZip)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	targetFileInfo, err := targetFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE, targetFileInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, targetFile)
	return err
}

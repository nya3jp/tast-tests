// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Goofy,
		Desc:     "Setup factory toolkit and exercise Goofy with custom TestList",
		Contacts: []string{"menghuan@chromium.org", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline"},
		Timeout:  3 * time.Minute,
	})
}

func Goofy(fullCtx context.Context, s *testing.State) {
	// testListName should match the filename of
	// "py/test/test_lists/generic_tast.test_list.json" under factory repo.
	// finishFlagFilePath should match the path which is hardcoded inside the
	// above TestList configuration file.
	const testListName = "generic_tast"
	const finishFlagFilePath = "/tmp/tast_factory_test"

	ctx, cancel := ctxutil.Shorten(fullCtx, 30*time.Second)
	defer cancel()

	setupFactory(ctx, s)
	defer cleanup(fullCtx, s)

	if err := setTestList(ctx, testListName); err != nil {
		s.Fatal("Failed to set TestList: ", err)
	}
	s.Logf("TestList is set to %s", testListName)

	// Make sure the temp file is not there.
	os.Remove(finishFlagFilePath)
	if err := testexec.CommandContext(ctx, "start", "factory").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start factory toolkit: ", err)
	}
	s.Log("factory toolkit started, waiting for TestList finished")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(finishFlagFilePath); err != nil {
			return errors.Errorf("finished flag file %s is missing", finishFlagFilePath)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Factory software fail to init and run. This high level and complicate tests is not a good choice to debug. Please check dumpped logs or other failed tests.  Failed to finish Goofy in time: ", err)
	}
}

func setupFactory(ctx context.Context, s *testing.State) {
	ver, err := installFactoryToolKit(ctx, s)
	if err != nil {
		s.Fatal("Install fail: ", err)
	}
	s.Logf("Installed factory toolkit with version: %s", ver)
}

func cleanup(ctx context.Context, s *testing.State) {
	s.Log("Start to backup factory logs under /var/log")

	logFiles := [3]string{"factory-init.log", "factory-session.log", "factory.log"}
	for _, logFile := range logFiles {
		src := filepath.Join("/var/log", logFile)
		dst := filepath.Join(s.OutDir(), logFile)
		if err := fsutil.CopyFile(src, dst); err != nil {
			s.Errorf("Failed to backup %s: %v", logFile, err)
		}
	}

	s.Log("Start to cleanup DUT")

	if err := stopGoofy(ctx); err != nil {
		s.Fatal("Failed to stop goofy when cleanup: ", err)
	}
	s.Log("Stopped Goofy")

	if err := uninstallFactoryToolKit(ctx); err != nil {
		s.Fatal("Failed to uninstall factory toolkit when cleanup: ", err)
	}
	s.Log("Uninstalled factory toolkit")
}

func installFactoryToolKit(ctx context.Context, s *testing.State) (version string, err error) {
	// the path should be synced with factory-mini.ebuild
	const toolkitInstallerPath = "/usr/local/factory-toolkit/install_factory_toolkit.run"

	if _, err := os.Stat(toolkitInstallerPath); err != nil {
		return "", errors.Wrapf(err, "failed to find factory toolkit installer: %s", toolkitInstallerPath)
	}

	// TODO(b/150189948): Support installing toolkit from artifacts
	// factory_image.zip
	return installFactoryToolKitFromToolkitInstaller(ctx, toolkitInstallerPath)
}

func installFactoryToolKitFromFactoryImage(ctx context.Context, imagePath string) (version string, err error) {
	// Create an temp directory.
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	// Unzip toolkit installer from image.
	if err := testexec.CommandContext(ctx, "unzip", imagePath, "toolkit/install_factory_toolkit.run", "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to unzip factory toolkit from factory_image.zip")
	}

	installerPath := filepath.Join(tempDir, "toolkit/install_factory_toolkit.run")
	return installFactoryToolKitFromToolkitInstaller(ctx, installerPath)
}

func installFactoryToolKitFromToolkitInstaller(ctx context.Context, installerPath string) (version string, err error) {
	if err := testexec.CommandContext(ctx, installerPath, "--", "--yes", "--no-enable").Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to install factory toolkit")
	}

	// Get the factory toolkit version.
	b, err := ioutil.ReadFile("/usr/local/factory/TOOLKIT_VERSION")
	if err != nil {
		return "", errors.Wrap(err, "failed to read version file")
	}
	version = strings.TrimSpace(string(b))
	return version, nil
}

func uninstallFactoryToolKit(ctx context.Context) error {
	return testexec.CommandContext(ctx, "factory_uninstall", "--yes").Run(testexec.DumpLogOnError)
}

// stopGoofy and cleanup all factory configuration.
func stopGoofy(ctx context.Context) error {
	// Cleanup the state files except logs.
	return testexec.CommandContext(ctx, "factory_restart", "-S", "-s", "-t", "-r").Run(testexec.DumpLogOnError)
}

// setTestList set the default TestList to `testListID` via factory test-list
// command.
func setTestList(ctx context.Context, testListID string) error {
	type DataFile struct {
		ID string `json:"id"`
	}
	const testListConfigPath = "/usr/local/factory/py/test/test_lists/active_test_list.json"
	var data DataFile

	if err := testexec.CommandContext(ctx, "factory", "test-list", testListID).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute `factory test-list` command")
	}

	// Verify the config content is correct.
	b, err := ioutil.ReadFile(testListConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to read TestList config file")
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return errors.Wrap(err, "failed to decode JSON")
	}
	if data.ID != testListID {
		return errors.Wrap(err, "testList config file doesn't contain the correct test id")
	}

	return nil
}

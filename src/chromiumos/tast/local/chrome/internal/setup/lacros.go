// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	// dataArtifact holds the name of the tarball which contains the lacros-chrome
	// binary.
	dataArtifact = "lacros_binary.tar"

	// LacrosSquashFSPath indicates the location of the rootfs lacros squashfs filesystem.
	LacrosSquashFSPath = "/opt/google/lacros/lacros.squash"

	// lacrosTestPath is the file path at which all lacros-chrome related test artifacts are stored.
	lacrosTestPath = "/usr/local/lacros_test_artifacts"

	// lacrosRootPath is the root directory for lacros-chrome related binaries.
	lacrosRootPath = lacrosTestPath + "/lacros_binary"
)

// EnsureLacrosForLaunch waits for Lacros to be provisioned and ready for launch.
func EnsureLacrosForLaunch(ctx context.Context, cfg config.LacrosConfig) (string, error) {
	testing.ContextLogf(ctx, "Waiting for Lacros ready with config: %+v", cfg)
	if cfg.SourceType == config.Deployed {
		// Skip preparing the lacros binary if it is already deployed for use.
		return cfg.SourcePath, nil
	}

	// Throw an error if lacros has been deployed, but the var lacrosDeployedBinary is unset.
	if cfg.SourceType == config.Omaha || cfg.SourceType == config.Rootfs {
		config, err := ioutil.ReadFile("/etc/chrome_dev.conf")
		if err == nil {
			for _, line := range strings.Split(string(config), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "--lacros-chrome-path") {
					return "", errors.New("found --lacros-chrome-path in /etc/chrome_dev.conf, but lacrosDeployedBinary is not specified")
				}
			}
		}
	}

	// Prepare the lacros binary if it isn't deployed already via lacrosDeployedBinary.
	var lacrosPath string
	switch cfg.SourceType {
	case config.External:
		if err := prepareLacrosBinary(ctx, cfg.SourcePath); err != nil {
			return "", errors.Wrap(err, "failed to prepare lacros-chrome")
		}
		lacrosPath = lacrosRootPath
	case config.Omaha:
		// When launched by Omaha we need to wait several seconds for lacros to be launchable.
		// It is ready when the image loader path is created with the chrome executable.
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		matches, err := waitForPathToExist(ctx, "/run/imageloader/lacros-dogfood*/*/chrome")
		if err != nil {
			return "", errors.Wrap(err, "failed to find lacros binary")
		}
		lacrosPath = filepath.Dir(matches[0])
	case config.Rootfs:
		// When launched from the rootfs partition, the lacros-chrome is already located
		// at /opt/google/lacros/lacros.squash in the OS, will be mounted at /run/lacros/.
		matches, err := waitForPathToExist(ctx, "/run/lacros/chrome")
		if err != nil {
			return "", errors.Wrap(err, "failed to find lacros binary")
		}
		lacrosPath = filepath.Dir(matches[0])
	default:
		return "", errors.New("Unrecognized mode: " + string(cfg.SourceType))
	}
	return lacrosPath, nil
}

// prepareLacrosBinary ensures that lacros-chrome binary is available on
// disk and ready to launch. Does not launch the binary.
// This will extract lacros-chrome to where the lacrosRootPath constant points to.
func prepareLacrosBinary(ctx context.Context, dataArtifactPath string) error {
	testing.ContextLog(ctx, "Preparing the environment to run Lacros")
	if err := os.RemoveAll(lacrosTestPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}

	if err := os.MkdirAll(lacrosTestPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	if err := os.Chown(lacrosTestPath, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return errors.Wrap(err, "failed to chown test artifacts directory")
	}

	testing.ContextLog(ctx, "Extracting lacros binary")
	tarCmd := testexec.CommandContext(ctx, "sudo", "-E", "-u", "chronos",
		"tar", "-xvf", dataArtifactPath, "-C", lacrosTestPath)

	if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to untar test artifacts")
	}

	if err := os.Chmod(lacrosRootPath, 0777); err != nil {
		return errors.Wrap(err, "failed to change permissions of the binary root dir path")
	}

	return nil
}

// waitForPathToExist is a helper method that waits the given binary path to be present
// then returns the matching paths or it will be timed out if the ctx's timeout is reached.
func waitForPathToExist(ctx context.Context, pattern string) (matches []string, err error) {
	return matches, testing.Poll(ctx, func(ctx context.Context) error {
		m, err := filepath.Glob(pattern)
		if err != nil {
			return errors.Wrapf(err, "binary path does not exist yet. expected: %v", pattern)
		}
		if len(m) == 0 {
			return errors.New("binary path does not exist yet. expected: " + pattern)
		}
		matches = append(matches, m...)
		return nil
	}, &testing.PollOptions{Interval: 2 * time.Second})
}

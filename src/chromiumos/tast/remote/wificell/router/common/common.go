// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
)

const (
	// Autotest may be used on these routers too, and if it failed to clean up, we may be out of space in /tmp.

	// AutotestWorkdirGlob is the path that grabs all autotest outputs.
	AutotestWorkdirGlob = "/tmp/autotest-*"
	// WorkingDir is the tast-test's working directory.
	WorkingDir = "/tmp/tast-test/"
)

// RouterCloseContextDuration is a shorter context.Context duration is used for
// running things before Router.Close to reserve time for it to run.
const RouterCloseContextDuration = 5 * time.Second

// RouterCloseFrameSenderDuration is the length of time the context deadline
// should be shortened by to reserve time for r.CloseFrameSender() to run.
const RouterCloseFrameSenderDuration = 2 * time.Second

// TimestampFileFormat is the time format used for timestamps in generated
// file names and folder names.
const TimestampFileFormat = "20060102-150405"

// BuildWorkingDirPath creates a working directory path based on the current
// time and base WorkingDir. All temporary files shall be placed within this
// directory during the life of the router controller instance. The time-based
// subdirectory under WorkingDir separates different instances' temporary files.
func BuildWorkingDirPath() string {
	return fmt.Sprintf("%s/%s/", WorkingDir, time.Now().Format(TimestampFileFormat))
}

// HostFileContentsMatch checks if the file at remoteFilePath on the remote
// host exists and that its contents match using the regex string matchRegex.
//
// Returns true if the file exists and its contents matches. Returns false
// with a nil error if the file does not exist.
func HostFileContentsMatch(ctx context.Context, host *ssh.Conn, remoteFilePath, matchRegex string) (bool, error) {
	// Verify that the file exists.
	fileExists, err := HostFileExists(ctx, host, remoteFilePath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check for the existence of file %q", remoteFilePath)
	}
	if !fileExists {
		return false, nil
	}

	// Verify that the file contents match.
	matcher, err := regexp.Compile(matchRegex)
	if err != nil {
		return false, errors.Wrapf(err, "failed to compile regex string %q", matchRegex)
	}
	fileContents, err := linuxssh.ReadFile(ctx, host, remoteFilePath)
	return matcher.Match(fileContents), nil
}

// HostFileExists checks the host for the file at remoteFilePath and returns
// true if remoteFilePath exists and is a regular file.
func HostFileExists(ctx context.Context, host *ssh.Conn, remoteFilePath string) (bool, error) {
	return HostTestPath(ctx, host, "-f", remoteFilePath)
}

// HostTestPath runs "test <testFlag> <remoteFilePath>" on the host and
// returns true if the test passes and false if the test fails.
func HostTestPath(ctx context.Context, host *ssh.Conn, testFlag, remotePath string) (bool, error) {
	if err := host.CommandContext(ctx, "test", testFlag, remotePath).Run(); err != nil {
		if err.Error() == "Process exited with status 1" {
			// Test was successfully evaluated and returned false.
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to run 'test %q %q' on remote host", testFlag, remotePath)
	}
	return true, nil
}

// RemoveDevicesWithPrefix removes the devices whose names start with the given prefix.
func RemoveDevicesWithPrefix(ctx context.Context, ipr *ip.Runner, prefix string) error {
	devs, err := ipr.LinkWithPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	for _, dev := range devs {
		if err := ipr.DeleteLink(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}

// HostapdSecurityConfigIsWEP returns true if the hostapd security configuration
// uses WEP.
func HostapdSecurityConfigIsWEP(secConf security.Config) (bool, error) {
	if secConf == nil {
		return false, nil
	}
	if secConf.Class() == shillconst.SecurityWEP {
		return true, nil
	}
	hostapdConfig, err := secConf.HostapdConfig()
	if err != nil {
		return false, errors.Wrap(err, "failed to build hostapd conf for security config")
	}
	if authAlgsStr, hasAuthAlgs := hostapdConfig["auth_algs"]; hasAuthAlgs {
		authAlgs, err := strconv.Atoi(authAlgsStr)
		if err == nil && (authAlgs == int(wep.AuthAlgoOpen) || authAlgs == int(wep.AuthAlgoShared)) {
			return true, nil
		}
	}
	for configOptionName := range hostapdConfig {
		if strings.Contains(configOptionName, "wep") {
			return true, nil
		}
	}
	return false, nil
}

// TearDownRedundantInterfaces tears down all the interfaces except those in linkList.
func TearDownRedundantInterfaces(ctx context.Context, ipr *ip.Runner, linkList []string) error {
	allLinks, err := ipr.ListUpLinks(ctx)
	if err != nil {
		return err
	}
re:
	for _, link := range allLinks {
		for _, upLink := range linkList {
			if link == upLink {
				continue re
			}
		}
		if err := ipr.SetLinkDown(ctx, link); err != nil {
			return err
		}
	}
	return nil
}

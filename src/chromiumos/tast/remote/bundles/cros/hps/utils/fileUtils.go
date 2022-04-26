// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains common api for those tests to use.
// fileutil contains functionality used by the HPS tast tests.
package utils

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	picFile       = "IMG_7451.jpg"
	noPersonFile  = "no-person-present.html"
	onePersonFile = "person-present.html"
	twoPeopleFile = "two-people-present.html"
)

// CreateTmpDir is to create dir under /tmp for storing tar files temporarily
func CreateTmpDir(ctx context.Context, dconn *ssh.Conn) (string, string, error) {
	powercycleTmpDir, err := dconn.CommandContext(ctx, "mktemp", "-d", "/tmp/powercycle_XXXXX").Output()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create test directory under /tmp for putting powercycle file")
	}
	powercycleDirPath := strings.TrimSpace(string(powercycleTmpDir))
	powercycleFilePath := filepath.Join(powercycleDirPath, hpsutil.P2PowerCycleFilename)
	return powercycleDirPath, powercycleFilePath, nil
}

// SendImageTar is to send pages with different presence to the remote tablet
func SendImageTar(ctx context.Context, dconn *ssh.Conn, originPath, powercycleDirPath, powercycleFilePath string) ([]string, error) {
	defer dconn.CommandContext(ctx, "rm", "-r", powercycleDirPath).Output()
	if _, err := linuxssh.PutFiles(
		ctx, dconn,
		map[string]string{
			originPath: powercycleFilePath,
		},
		linuxssh.DereferenceSymlinks,
	); err != nil {
		return nil, errors.Wrapf(err, "failed to send data to remote data path %v", powercycleFilePath)
	}
	testing.ContextLog(ctx, "Sending file to dut, path being: ", powercycleFilePath)

	dirPath := filepath.Dir(originPath)
	testing.ContextLog(ctx, "dirpath: ", dirPath)

	tarOut, err := testexec.CommandContext(ctx, "tar", "--strip-components=1", "-xvf", originPath, "-C", dirPath).Output()
	testing.ContextLog(ctx, "Extracting following files: ", string(tarOut))
	if err != nil {
		return nil, errors.Wrap(err, "failed to untar test artifacts")
	}

	picture := filepath.Join(dirPath, picFile)
	chartPaths := []string{
		filepath.Join(dirPath, noPersonFile),
		filepath.Join(dirPath, onePersonFile),
		filepath.Join(dirPath, twoPeopleFile)}

	filePaths := append(chartPaths, picture)

	return filePaths, nil
}

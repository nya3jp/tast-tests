// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// PersistableToken derives the token from the mount path. This is used
// to identify the user account directory under ~/GCache/v2.
func PersistableToken(mountPath string) string {
	return strings.TrimPrefix(mountPath, "/media/fuse/drivefs-")
}

// ConfigPath returns the path to `elem...` in the DriveFS
// configuration directory based on the provided `homeDir` and
// `persistableToken`.
func ConfigPath(homeDir, persistableToken string, elem ...string) string {
	return path.Join(append([]string{homeDir, "GCache", "v2", persistableToken}, elem...)...)
}

// SaveDriveLogsOnError dumps the drivefs logs to the context output dir to
// enable ease of debugging when an error occurs. This should be deferred on any
// test that you wish to obtain logs on error.
func SaveDriveLogsOnError(ctx context.Context, hasError func() bool, normalizedUser, mountPath string) {
	if !hasError() {
		return
	}
	persistableToken := PersistableToken(mountPath)
	if len(persistableToken) == 0 {
		testing.ContextLog(ctx, "Could not obtain the drive persistable token from mount path: ", mountPath)
		return
	}
	homeDir, err := cryptohome.UserPath(ctx, normalizedUser)
	if err != nil {
		testing.ContextLog(ctx, "Could not obtain the home dir path: ", err)
		return
	}
	saveDriveLogs(ctx, homeDir, persistableToken)
}

func saveDriveLogs(ctx context.Context, homeDir, persistableToken string) {
	driveLogPath := ConfigPath(homeDir, persistableToken, "Logs", "drivefs.txt")
	logContents, err := ioutil.ReadFile(driveLogPath)
	if err != nil {
		testing.ContextLogf(ctx, "Could not read the Drive log %q: %v", driveLogPath, err)
		return
	}
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		testing.ContextLog(ctx, "Could not obtain the context out dir")
		return
	}
	if err = ioutil.WriteFile(filepath.Join(outDir, "drivefs_logs.txt"), logContents, 0644); err != nil {
		testing.ContextLog(ctx, "Could not write the Drive log to out dir: ", err)
	}
}

// GenerateTestFileName generates a unique-ish file name based on a provided
// prefix, the current time, and a random number.
func GenerateTestFileName(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), rand.Intn(10000))
}

// MD5SumFile generates an MD5 sum of a file at `path`.
func MD5SumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

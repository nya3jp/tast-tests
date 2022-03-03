// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// Variables used by other tast tests
const (
	DefaultUITimeout = 20 * time.Second
)

// WaitForFileSaved waits for the presence of the captured file with file name matching the specified
// change timeout from 5s to 60s
// refer to cca.go in pacakge cca
// pattern, size larger than zero, and modified time after the specified timestamp.
func WaitForFileSaved(ctx context.Context, dir string, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	const timeout = time.Minute
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read the camera directory")
		}
		for _, file := range files {
			if file.Size() == 0 || file.ModTime().Before(ts) {
				continue
			}
			if _, ok := seen[file.Name()]; ok {
				continue
			}
			seen[file.Name()] = struct{}{}
			testing.ContextLog(ctx, "New file found: ", file.Name())
			if pat.MatchString(file.Name()) {
				testing.ContextLog(ctx, "Found a match: ", file.Name())
				result = file
				return nil
			}
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrapf(err, "no matching output file found after %v", timeout)
	}
	return result, nil
}

// PrettyPrint print pretty
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func copyUsbFile(ctx context.Context, s *testing.State, filename string) error {

	// through usb

	// to get usb path, sth like /media/removable/{$usbName}
	getUsbPath := testexec.CommandContext(ctx, "sh", "-c", "sudo lsblk -l -o mountpoint | grep removable")
	usbPath, err := getUsbPath.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(getUsbPath.Args))
	}

	// copy file from usb to "Downloads" folder
	copyFile := testexec.CommandContext(ctx, "cp",
		filepath.Join(strings.TrimSpace(string(usbPath)), filename),
		filesapp.DownloadPath)

	if err = copyFile.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(copyFile.Args))
	}

	return nil
}

// CopyFileToServer copy file from chromebook to fixture server
func CopyFileToServer(ctx context.Context, s *testing.State, chromebookPath string) error {
	return nil

	// retrieve filename
	_, filename := filepath.Split(chromebookPath)

	// transfer file to tast env
	dir, ok := testing.ContextOutDir(ctx)
	if ok && dir != "" {
		if _, err := os.Stat(dir); err == nil {
			testing.ContextLogf(ctx, "copy file to %s", dir)

			// read file
			b, err := ioutil.ReadFile(chromebookPath)
			if err != nil {
				return err
			}

			// write tastPath to result folder
			tastPath := filepath.Join(s.OutDir(), filename)
			if err := ioutil.WriteFile(tastPath, b, 0644); err != nil {
				return errors.Wrapf(err, "failed to dump bytes to %s", tastPath)
			}
		}
	}

	return nil
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// SuspendChromebook suspend then reconnect chromebook for 15s
func SuspendChromebook(ctx context.Context, s *testing.State, cr *chrome.Chrome) (*chrome.TestConn, error) {

	s.Log("Suspend 15s then reconnect chromebook")

	command := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s %s", "powerd_dbus_suspend", "--suspend_for_sec=15"))

	if err := command.Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to execute powerd_dbus_suspend command")
	}

	// reconnect chrome
	if err := cr.Reconnect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reconnect to chrome")
	}

	// re-build API connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return tconn, nil
}

// PoweroffChromebook notice: if chromebook was powered off, then cause SSH lost
func PoweroffChromebook(ctx context.Context, s *testing.State) error {

	s.Log("Power off chromebook")

	err := testexec.CommandContext(ctx, "shutdown", "-P", "now").Run(testexec.DumpLogOnError)
	// err := testexec.CommandContext(ctx, "shutdown", "-P", "now").Start()
	if err != nil {
		return errors.Wrap(err, "failed to execute power off chromebook")
	}

	return nil
}

// CopyFile copy file from chromebook to tast server
func CopyFile(ctx context.Context, s *testing.State, inputFile string) error {

	// retrieve filename
	_, filename := filepath.Split(inputFile)

	// transfer file to tast env
	dir, ok := testing.ContextOutDir(ctx)
	if ok && dir != "" {
		if _, err := os.Stat(dir); err == nil {
			testing.ContextLogf(ctx, "copy file to %s", dir)

			// read file
			b, err := ioutil.ReadFile(inputFile)
			if err != nil {
				return err
			}

			// write outputFile to result folder
			outputFile := filepath.Join(s.OutDir(), filename)
			if err := ioutil.WriteFile(outputFile, b, 0644); err != nil {
				return errors.Wrapf(err, "failed to dump bytes to %s", outputFile)
			}
		}
	}

	return nil
}

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
			return errors.Wrap(err, "failed to read the directory")
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
func PrettyPrint(ctx context.Context, i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	testing.ContextLog(ctx, string(s))

}

// CopyUsbFile copy usb file to downloads folder
func CopyUsbFile(ctx context.Context, s *testing.State, filename string) error {

	// to get usb path, path like /media/removable/{$usbName}
	getUsbPath := testexec.CommandContext(ctx, "sudo lsblk -l -o mountpoint | grep removable")
	usbPath, err := getUsbPath.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(getUsbPath.Args))
	}

	// copy file from usb to "Downloads" folder
	copyUsbFile := testexec.CommandContext(ctx, "cp",
		filepath.Join(strings.TrimSpace(string(usbPath)), filename),
		filesapp.DownloadPath)

	if err = copyUsbFile.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(copyUsbFile.Args))
	}

	return nil
}

// RunOrFatal runOrFatal runs body as subtest, then invokes s.Fatal if it returns an error
func RunOrFatal(ctx context.Context, s *testing.State, name string, body func(context.Context, *testing.State) error) bool {
	return s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		if err := body(ctx, s); err != nil {
			s.Fatal("subtest failed: ", err)
		}
	})
}

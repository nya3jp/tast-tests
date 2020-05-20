// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpacli contains funtions running wpa_cli command.
package wpacli

import (
	"context"
	"io/ioutil"
	"path"
	"strings"

	"chromiumos/tast/common/network/cmd"
)

// WpaCli holds methods to interact with wpa_cli command.
type WpaCli struct {
	cmd cmd.Runner
}

// New returns a WpaCli object.
func New(c cmd.Runner) *WpaCli {
	return &WpaCli{cmd: c}
}

// sudoWpaCli returns a sudo command args that runs wpa_cli with args under sudo.
func sudoWpaCli(args ...string) []string {
	ret := []string{"-u", "wpa", "-g", "wpa", "wpa_cli"}
	for _, arg := range args {
		ret = append(ret, arg)
	}
	return ret
}

// Run rusn a wpa_cli command with args and waits for its completion.
func (w *WpaCli) Run(ctx context.Context, args ...string) error {
	return w.cmd.Run(ctx, "sudo", sudoWpaCli(args...)...)
}

// Output runs a wpa_cli command with args, waits for its completion and returns stdout output of the command.
func (w *WpaCli) Output(ctx context.Context, args ...string) ([]byte, error) {
	return w.cmd.Output(ctx, "sudo", sudoWpaCli(args...)...)
}

// MayOutputToFile writes cmdOut to file if cmdOut is multiline; otherwise, returns cmdOut.
// Output filename: outDir/wpa_cli.log
func (w *WpaCli) MayOutputToFile(cmdOut []byte, outDir string) string {
	ret := strings.TrimSpace(string(cmdOut))
	if strings.Contains(ret, "\n") {
		path := path.Join(outDir, "wpa_cli.log")
		ioutil.WriteFile(path, cmdOut, 0644)
		return "check wpa_cli logfile: " + path
	}
	return ret
}

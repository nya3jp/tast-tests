// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpacli

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
)

// Runner contains methods involving wpa_cli command.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates a new wpa_cli command utility runner.
func NewRunner(c cmd.Runner) *Runner {
	return &Runner{cmd: c}
}

// sudoWpacli returns a sudo command args that runs wpa_cli with args under sudo.
func sudoWpacli(args ...string) []string {
	ret := []string{"-u", "wpa", "-g", "wpa", "wpa_cli"}
	for _, arg := range args {
		ret = append(ret, arg)
	}
	return ret
}

// Ping runs "wpa_cli -i iface ping" command and expects to see PONG.
func (r *Runner) Ping(ctx context.Context, iface string) ([]byte, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWpacli("-i", iface, "ping")...)
	if err != nil {
		return cmdOut, errors.Wrapf(err, "failed running wpa_cli -i %s ping", iface)
	}
	if !strings.Contains(string(cmdOut), "PONG") {
		return cmdOut, errors.New("failed to see 'PONG' in wpa_cli ping output")
	}
	return cmdOut, nil
}

// MayOutputToFile writes cmdOut to path if cmdOut is multiline; otherwise, returns cmdOut.
func MayOutputToFile(cmdOut []byte, outDir, filename string) string {
	ret := strings.TrimSpace(string(cmdOut))
	if strings.Contains(ret, "\n") {
		outPath := path.Join(outDir, filename)
		err := ioutil.WriteFile(outPath, cmdOut, 0644)
		if err != nil {
			return fmt.Sprintf("failed to write output to %s: %s", outPath, err)
		}
		return "output file: " + path.Join("/tmp/tast/results/latest/tests", path.Base(outDir), filename)
	}
	return ret
}

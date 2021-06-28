// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package syzutils contains functionality shared by tests that
// exercise syzcorpus.
package syzutils

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

var crashPatterns = []string{
	"BUG: ",
	"INFO: ",
	"PANIC: ",
	"WARNING: ",
	"Kernel panic",
	"general protection fault",
	"divide error: ",
	"Internal error: ",
	"Unhandled fault:",
	"Alignment trap:",
	"invalid opcode:",
	"stack segment: ",
	"Unable to handle kernel ",
}

// FindDUTArch determines the DUT arch.
func FindDUTArch(ctx context.Context, d *dut.DUT) (string, error) {
	arch, err := d.Command("uname", "-m").Output(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(arch)), nil
}

// WarningInDmesg checks for an error or warning in the dmesg log. If an
// error or warning is found, the contents of the log are returned.
func WarningInDmesg(ctx context.Context, d *dut.DUT) ([]byte, error) {
	testing.ContextLog(ctx, "Checking for warning in dmesg")
	contents, err := readDmesg(ctx, d)
	if err != nil {
		return nil, err
	}
	for _, pattern := range crashPatterns {
		if bytes.Contains(contents, []byte(pattern)) {
			testing.ContextLogf(ctx, "pattern %q matched", pattern)
			return contents, nil
		}
	}
	return nil, nil
}

func readDmesg(ctx context.Context, d *dut.DUT) ([]byte, error) {
	contents, err := d.Command("dmesg").Output(ctx)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

// ClearDmesg clears the dmesg log.
func ClearDmesg(ctx context.Context, d *dut.DUT) error {
	testing.ContextLog(ctx, "Clearing dmesg")
	if err := d.Conn().Command("dmesg", "--clear").Run(ctx); err != nil {
		testing.ContextLog(ctx, "Unable to clear dmesg: ", err)
		return err
	}
	return nil
}

// MkdirRemote creates a directory at path on the DUT.
func MkdirRemote(ctx context.Context, d *dut.DUT, path string) error {
	if err := d.Conn().Command("mkdir", "-p", path).Run(ctx); err != nil {
		return err
	}
	return nil
}

// RmdirRemote recursively removes the directory at path on the DUT.
func RmdirRemote(ctx context.Context, d *dut.DUT, path string) error {
	if err := d.Conn().Command("rm", "-rf", path).Run(ctx); err != nil {
		return err
	}
	return nil
}

// CopyRepro copies a repro file to the DUT.
func CopyRepro(ctx context.Context, d *dut.DUT, localPath, remotePath string) error {
	testing.ContextLogf(ctx, "Copying %v to %v", localPath, fmt.Sprintf("root@DUT:%v", remotePath))
	if _, err := linuxssh.PutFiles(
		ctx,
		d.Conn(),
		map[string]string{localPath: remotePath},
		linuxssh.DereferenceSymlinks,
	); err != nil {
		return err
	}
	return nil
}

// RunRepro runs the repro present at remotePath on the DUT with a specified timeout.
func RunRepro(ctx context.Context, d *dut.DUT, remotePath string, timeout time.Duration) ([]byte, error) {
	testing.ContextLog(ctx, "Going to run repro")
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run the remote command with a timeout of `timeout`. KILL signal will also be
	// sent after `timeout`.
	cmd := d.Conn().Command("timeout", "-k", "2", fmt.Sprintf("%v", timeout.Seconds()), filepath.Join(remotePath))
	// The repro might exit with a non-zero exit code and this is expected. The repro
	// might also run indefinitely, and be terminated by the context timeout.
	if out, err := cmd.CombinedOutput(ctx); err != nil {
		return out, err
	}
	return nil, nil
}

// ExtractCorpus unzips the zip file pointed to by dataPath into tastDir.
func ExtractCorpus(ctx context.Context, tastDir, dataPath string) error {
	cmd := exec.CommandContext(ctx, "unzip", dataPath, "-d", tastDir)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to extract corpus")
	}
	return nil
}

// LoadEnabledRepros reads and returns a list of repros from the
// provided input filepath.
func LoadEnabledRepros(fpath string) (map[string]bool, error) {
	contents, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	enabledRepros := make(map[string]bool)
	for _, fname := range strings.Split(string(contents), "\n") {
		fname = strings.TrimSpace(fname)
		if fname == "" || strings.HasPrefix(fname, "#") {
			continue
		}
		enabledRepros[fname] = true
	}
	return enabledRepros, nil
}

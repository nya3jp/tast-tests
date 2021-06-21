// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package syzutils contains functionality shared by tests that
// exercise syzcorpus.
package syzutils

import (
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

// ProcessNameLength denotes the length of the process name used to match
// against processes with pkill. This is the length of the process name inside
// /proc/pid/stat.
const ProcessNameLength = 15

// FindDUTArch determines the DUT arch.
func FindDUTArch(ctx context.Context, d *dut.DUT) (string, error) {
	arch, err := d.Command("uname", "-m").Output(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(arch)), nil
}

// WarningInDmesg checks for an error or warning in the dmesg log.
func WarningInDmesg(ctx context.Context, d *dut.DUT) (bool, error) {
	testing.ContextLog(ctx, "Checking for warning in dmesg")
	contents, err := readDmesg(ctx, d)
	if err != nil {
		return false, err
	}
	for _, pattern := range crashPatterns {
		if strings.Contains(contents, pattern) {
			testing.ContextLogf(ctx, "pattern %q matched", pattern)
			return true, nil
		}
	}
	return false, nil
}

func readDmesg(ctx context.Context, d *dut.DUT) (string, error) {
	contents, err := d.Command("dmesg").Output(ctx)
	if err != nil {
		return "", err
	}
	return string(contents), nil
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

	cmd := d.Conn().Command(filepath.Join(remotePath))
	// The repro might exit with a non-zero exit code and this is expected. The repro
	// might also run indefinitely, and be terminated by the context timeout.
	if out, err := cmd.CombinedOutput(ctx); err != nil {
		return out, err
	}
	return nil, nil
}

// KillRepro kills the process to ensure it is not running in the background.
func KillRepro(ctx context.Context, d *dut.DUT, fname string) error {
	testing.ContextLog(ctx, "Killing process")
	// Some processes are not killed readily with a single pkill, so continue
	// retrying the pkill.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Repro process is still running, issue a pkill.
		if err := d.Conn().Command("pkill", "-9", fname[:ProcessNameLength]).Run(ctx); err != nil {
			testing.ContextLog(ctx, "pkill failed: ", err)
		}
		// If the repro process is not running, return.
		exists, err := processExists(ctx, d, fname)
		if err != nil || exists {
			return errors.Errorf("pkill retry required(exists=%v, err=%v)", exists, err)
		}
		return nil
	}, &testing.PollOptions{Interval: 2 * time.Second}); err != nil {
		return err
	}
	return nil
}

func processExists(ctx context.Context, d *dut.DUT, fname string) (bool, error) {
	out, err := d.Conn().Command("ps", "-A").Output(ctx)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(out), fname[:ProcessNameLength]) {
		return true, nil
	}
	return false, nil
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

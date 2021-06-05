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
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// TestContext implements utility functions for all tests within
// syzcorpus.
type TestContext struct {
	ctx       context.Context
	d         *dut.DUT
	dutSSHKey string
}

var boardArchMapping = map[string]string{
	"octopus":  "amd64",
	"dedede":   "amd64",
	"nautilus": "amd64",
	// trogdor has an arm userspace.
	"trogdor": "arm",
}

// NewTestContext creates and returns an instance of TestContext.
func NewTestContext(ctx context.Context, d *dut.DUT, dutSSHKey string) *TestContext {
	return &TestContext{
		ctx:       ctx,
		d:         d,
		dutSSHKey: dutSSHKey,
	}
}

// FindSyzkallerArch determines the userspace type for the DUT.
func (tc *TestContext) FindSyzkallerArch() (string, error) {
	board, err := reporters.New(tc.d).Board(tc.ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to find board")
	}
	if _, ok := boardArchMapping[board]; !ok {
		return "", errors.Wrapf(err, "unexpected board: %v", board)
	}
	return boardArchMapping[board], nil
}

// ResetDUT restatrs the device, and remounts /tmp as rwx if specified.
func (tc *TestContext) ResetDUT(s *testing.State, remount bool) error {
	// Reboot the DUT.
	if err := tc.RebootDUT(s); err != nil {
		return err
	}

	// Wait for the device to come back up.
	tc.ensureDUTReady(s)

	// Establish a tast ssh session.
	if err := tc.d.Connect(tc.ctx); err != nil {
		return err
	}

	// Mount /tmp as rwx.
	if remount {
		return tc.RemountTmp(s)
	}
	return nil
}

// RebootDUT reboots the DUT and waits for the device to be ready.
func (tc *TestContext) RebootDUT(s *testing.State) error {
	s.Log("Rebooting device")
	if err := tc.d.Conn().Command("reboot").Run(tc.ctx); err != nil {
		return err
	}
	testing.Sleep(tc.ctx, 3*time.Second)
	for {
		if err := tc.isDUTReady(s); err != nil {
			return nil
		}
	}
}

// RemountTmp remounts /tmp as rwx.
func (tc *TestContext) RemountTmp(s *testing.State) error {
	s.Log("Remounting /tmp as exec")
	if err := tc.d.Conn().Command("mount", "-o", "remount,rw", "-o", "exec", "/tmp").Run(tc.ctx); err != nil {
		return errors.Wrap(err, "unable to remount /tmp as rwx")
	}
	return nil
}

func (tc *TestContext) ensureDUTReady(s *testing.State) {
	start := time.Now()
	for {
		if err := tc.isDUTReady(s); err != nil {
			s.Log("Waiting for device to come back: ", err)
			testing.Sleep(tc.ctx, 5*time.Second)
			continue
		}
		s.Logf("Device is up: took %v secs", time.Since(start))
		return
	}
}

func (tc *TestContext) isDUTReady(s *testing.State) error {
	ret := strings.Split(tc.d.HostName(), ":")
	host, port := ret[0], ret[1]
	var stderr bytes.Buffer
	cmd := exec.Command(
		"timeout", "5", "ssh",
		"-p", port,
		"-i", s.DataPath(tc.dutSSHKey),
		fmt.Sprintf("root@%v", host), "pwd",
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "dut not ready: %v", stderr.String())
	}
	return nil
}

// WarningInDmesg checks for an error or warning in the dmesg log.
func (tc *TestContext) WarningInDmesg(s *testing.State) (bool, error) {
	s.Log("Checking for warning in dmesg")
	contents, err := tc.readDmesg()
	if err != nil {
		return false, err
	}
	// TODO: Allow for using syzkaller's crashlog parsing to check for crashes.
	if strings.Contains(contents, "WARNING") || strings.Contains(contents, "segfault") {
		return true, nil
	}
	return false, nil
}

func (tc *TestContext) readDmesg() (string, error) {
	contents, err := tc.d.Command("dmesg").Output(tc.ctx)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

// ClearDmesg clears the dmesg log.
func (tc *TestContext) ClearDmesg(s *testing.State) error {
	s.Log("Clearning dmesg")
	if err := tc.d.Conn().Command("dmesg", "--clear").Run(tc.ctx); err != nil {
		s.Log("Unable to clear dmesg: ", err)
		return err
	}
	return nil
}

// CopyRepro copies a repro file to the DUT.
func (tc *TestContext) CopyRepro(s *testing.State, localPath, remotePath string) error {
	s.Logf("copying %v to %v", localPath, fmt.Sprintf("root@DUT:%v", remotePath))
	if _, err := linuxssh.PutFiles(
		tc.ctx,
		tc.d.Conn(),
		map[string]string{localPath: remotePath},
		linuxssh.DereferenceSymlinks,
	); err != nil {
		return err
	}
	return nil
}

// RunRepro runs the repro present at remotePath on the DUT with a specified timeout.
func (tc *TestContext) RunRepro(s *testing.State, remotePath string, timeout time.Duration) error {
	s.Log("going to run repro")
	cmd := tc.d.Conn().Command(filepath.Join(remotePath))
	if err := cmd.Start(tc.ctx); err != nil {
		return err
	}
	func() {
		defer cmd.Wait(tc.ctx, ssh.DumpLogOnError)
		s.Log("waiting for command to finish")
		testing.Sleep(tc.ctx, timeout)
		s.Log("... stopping running repro")
		cmd.Abort()
	}()
	return nil
}

// ExtractCorpus unzips the zip file pointed to by dataPath into tastDir.
func ExtractCorpus(tastDir, dataPath string) error {
	cmd := exec.Command("unzip", dataPath, "-d", tastDir)
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
		if strings.HasPrefix(fname, "#") || len(strings.TrimSpace(fname)) == 0 {
			continue
		}
		enabledRepros[fname] = true
	}
	return enabledRepros, nil
}

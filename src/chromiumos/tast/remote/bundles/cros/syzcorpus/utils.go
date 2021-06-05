// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syzcorpus

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

type TestContext struct {
	ctx context.Context
	s   *testing.State
	d   *dut.DUT
}

var boardArchMapping = map[string]string{
	"octopus":  "amd64",
	"dedede":   "amd64",
	"nautilus": "amd64",
	// trogdor has an arm userspace.
	"trogdor": "arm",
}

func NewTestContext(ctx context.Context, s *testing.State) *TestContext {
	return &TestContext{
		ctx: ctx,
		s:   s,
		d:   s.DUT(),
	}
}

func (tc *TestContext) findSyzkallerArch() (string, error) {
	board, err := reporters.New(tc.d).Board(tc.ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to find board")
	}
	if _, ok := boardArchMapping[board]; !ok {
		return "", errors.Wrapf(err, "unexpected board: %v", board)
	}
	return boardArchMapping[board], nil
}

func (tc *TestContext) resetDUT(remount bool) error {
	// Reboot the DUT.
	if err := tc.rebootDUT(); err != nil {
		return err
	}

	// Wait for the device to come back up.
	tc.ensureDUTReady()

	// Establish a tast ssh session.
	if err := tc.d.Connect(tc.ctx); err != nil {
		return err
	}

	// Mount /tmp as rwx.
	if remount {
		return tc.remountTmp()
	}
	return nil
}

func (tc *TestContext) rebootDUT() error {
	tc.s.Log("Rebooting device")
	if err := tc.d.Conn().Command("reboot").Run(tc.ctx); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	for {
		if err := tc.isDUTReady(); err != nil {
			return nil
		}
	}
}

func (tc *TestContext) remountTmp() error {
	tc.s.Log("Remounting /tmp as exec")
	if err := tc.d.Conn().Command("mount", "-o", "remount,rw", "-o", "exec", "/tmp").Run(tc.ctx); err != nil {
		return errors.Wrapf(err, "unable to remount /tmp as rwx")
	}
	return nil
}

func (tc *TestContext) ensureDUTReady() {
	start := time.Now()
	for {
		if err := tc.isDUTReady(); err != nil {
			tc.s.Logf("Waiting for device to come back: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		tc.s.Logf("Device is up: took %v secs", time.Since(start))
		return
	}
}

func (tc *TestContext) isDUTReady() error {
	ret := strings.Split(tc.d.HostName(), ":")
	host, port := ret[0], ret[1]
	var stderr bytes.Buffer
	cmd := exec.Command(
		"timeout", "5", "ssh",
		"-p", port,
		"-i", tc.s.DataPath(DUT_SSH_KEY),
		fmt.Sprintf("root@%v", host), "pwd",
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "dut not ready: %v", stderr.String())
	}
	return nil
}

func (tc *TestContext) warningInDmesg() (bool, error) {
	tc.s.Log("Checking for warning in dmesg")
	contents, err := tc.readDmesg()
	if err != nil {
		return false, err
	}
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

func (tc *TestContext) clearDmesg() error {
	tc.s.Log("Clearning dmesg")
	if err := tc.d.Conn().Command("dmesg", "--clear").Run(tc.ctx); err != nil {
		tc.s.Logf("Unable to clear dmesg: %v", err)
		return err
	}
	return nil
}

func (tc *TestContext) copyRepro(localPath, remotePath string) error {
	tc.s.Logf("copying %v to %v", localPath, fmt.Sprintf("root@DUT:%v", remotePath))
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

func (tc *TestContext) runRepro(remotePath string, timeout time.Duration) error {
	tc.s.Log("going to run repro")
	cmd := tc.d.Conn().Command(filepath.Join(remotePath))
	if err := cmd.Start(tc.ctx); err != nil {
		return err
	}
	func() {
		defer cmd.Wait(tc.ctx, ssh.DumpLogOnError)
		tc.s.Log("waiting for command to finish...")
		testing.Sleep(tc.ctx, timeout)
		tc.s.Log("... stopping running repro")
		cmd.Abort()
	}()
	return nil
}

func extractCorpus(tastDir, dataPath string) error {
	cmd := exec.Command("unzip", dataPath, "-d", tastDir)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to extract corpus: %v")
	}
	return nil
}

func loadEnabledRepros(fpath string) (map[string]bool, error) {
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

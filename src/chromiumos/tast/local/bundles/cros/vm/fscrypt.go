// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const runFscrypt string = "run-fscrypt.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fscrypt,
		Desc:         "Tests that virtio-fs supports directory encryption",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
		Data:         []string{runFscrypt},
		SoftwareDeps: []string{"android_vm"},
	})
}

func addKey(ctx context.Context) (string, error) {
	cmd := testexec.CommandContext(ctx, "e4crypt", "add_key")
	cmd.Stdin = strings.NewReader("test0000")

	buf, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run command")
	}

	var key string
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		if _, err := fmt.Sscanf(scanner.Text(), "Added key with descriptor [%s]", &key); err == nil {
			break
		}

		if _, err := fmt.Sscanf(scanner.Text(), "Key with descriptor [%s] already exists", &key); err == nil {
			break
		}
	}

	if key == "" {
		return "", errors.New("unable to read key descriptor")
	}

	// The %s specifier is greedy and matches up to the first whitespace character so
	// we have to manually strip off the trailing ']' character here.
	return key[:len(key)-1], nil
}

func setPolicy(ctx context.Context, key, dir string) error {
	cmd := testexec.CommandContext(ctx, "e4crypt", "set_policy", key, dir)
	return cmd.Run(testexec.DumpLogOnError)
}

func getPolicy(ctx context.Context, dir string) (string, error) {
	cmd := testexec.CommandContext(ctx, "e4crypt", "get_policy", dir)

	buf, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run command")
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), dir) {
			continue
		}

		fields := strings.Fields(scanner.Text())
		return fields[len(fields)-1], nil
	}

	return "", errors.New("unable to read encryption policy key descriptor")
}

func Fscrypt(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.Fscrypt.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	encrypted := path.Join(td, "encrypted")
	if err := os.Mkdir(encrypted, 0755); err != nil {
		s.Fatal("Failed to create test encrypted directory: ", err)
	}

	key, err := addKey(ctx)
	if err != nil {
		s.Fatal("Failed to add encryption key: ", err)
	}

	if err := setPolicy(ctx, key, encrypted); err != nil {
		s.Fatal("Failed to set encryption policy for test directory: ", err)
	}

	logFile := filepath.Join(s.OutDir(), "serial.log")

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runFscrypt)),
		"--",
		td,
	}

	// We use the arcvm kernel for this test because for now that's the only one where
	// this needs to work.
	args := []string{
		"--nofile=262144",
		"crosvm", "run",
		"-p", strings.Join(params, " "),
		"-c", "1",
		"-m", "256",
		"-s", td,
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"/opt/google/vms/android/vmlinux",
	}

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	cmd := testexec.CommandContext(ctx, "prlimit", args...)
	if cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	log, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Fatal("Failed to read serial log: ", err)
	}

	var actualKey string
	lines := strings.Split(string(log), "\n")

	// The lines we care about are at the end of the log so iterate over it in reverse.
	for idx := len(lines) - 1; idx >= 0; idx-- {
		if _, err := fmt.Sscanf(lines[idx], "Encrypted directory key: %s", &actualKey); err == nil {
			break
		}
	}

	if actualKey == "" {
		s.Fatal("Failed to get encryption policy from VM")
	}

	if actualKey != key {
		s.Fatalf("Encryption key in guest doesn't match host key: want %s, got %s", key, actualKey)
	}

	newdir := path.Join(td, "newdir")
	if _, err := os.Stat(newdir); os.IsNotExist(err) {
		s.Fatal("Failed to create new directory inside VM")
	}

	guestKey, err := getPolicy(ctx, newdir)
	if err != nil {
		s.Fatal("Failed to get encryption policy set by VM: ", err)
	}

	if guestKey != key {
		s.Fatalf("Encryption key used by guest doesn't match host key: want %s, got %s", key, guestKey)
	}
}

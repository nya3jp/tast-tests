// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package verity provides variation of dm-verity tests and its utilities.
package verity

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	blockSize = 4096

	// The number of blocks of the test image file.
	nTestBlocks = 100
)

// releaseDevice releases the device by executing the given command.
// Note that there could be a chance some other thing keeps the device open.
// Considering the case, retries for 1 sec.
func releaseDevice(ctx context.Context, c []string, device string) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, c[0], c[1:]...).Run()
	}, &testing.PollOptions{Timeout: time.Second})
	if err == nil {
		// Succeeded.
		return nil
	}

	// Build an error message useful for debugging.
	cmd := testexec.CommandContext(ctx, "fuser", "-v", device)
	fuser, ferr := cmd.Output()
	if ferr != nil {
		// fuser is just for logging purpose, so ignore an error.
		testing.ContextLog(ctx, "Failed to call fuser: ", ferr)
		cmd.DumpLog(ctx)
	}

	cmd = testexec.CommandContext(ctx, "lsblk", device)
	lsblk, lerr := cmd.Output()
	if err != nil {
		// lsblk is just for logging purpose, so ignore an error.
		testing.ContextLog(ctx, "Failed to call lsblk: ", lerr)
		cmd.DumpLog(ctx)
	}

	return errors.Wrapf(err, "failed to release %s, fuser=%s, lsblk=%s", device, fuser, lsblk)
}

// createImage creates a temporary file for testing under dir.
// The size of the file will be nBlocks * blockSize bytes of 0.
// The path to the created image is returned. Callers have responsibility to
// delete the file when it gets no longer needed.
func createImage(ctx context.Context, dir, name string, nBlocks uint) (string, error) {
	f, err := ioutil.TempFile(dir, name+".img.")
	if err != nil {
		return "", err
	}
	defer func() {
		if f != nil {
			os.Remove(f.Name())
		}
	}()
	if err := f.Close(); err != nil {
		return "", err
	}
	cmd := testexec.CommandContext(
		ctx, "dd", "if=/dev/zero", "of="+f.Name(),
		fmt.Sprintf("bs=%d", blockSize), "count=0",
		fmt.Sprintf("seek=%d", nBlocks))
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	ret := f.Name()
	f = nil
	return ret, nil
}

// createFileSystem creates a file system on |path|.
func createFileSystem(ctx context.Context, path string) error {
	cmd := testexec.CommandContext(
		ctx, "mkfs.ext3", "-b", strconv.Itoa(blockSize), "-F", path)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// createHash generates the hash for verity. On success, the path to the hash
// file and the device mapper table are returned.
func createHash(ctx context.Context, dir, name, image string, nBlocks uint) (hash, table string, err error) {
	f, err := ioutil.TempFile(dir, name+".hash.")
	if err != nil {
		return "", "", err
	}
	defer func() {
		if f != nil {
			os.Remove(f.Name())
		}
	}()
	if err := f.Close(); err != nil {
		return "", "", err
	}
	cmd := testexec.CommandContext(
		ctx, "verity", "mode=create", "alg=sha1",
		"payload="+image, fmt.Sprintf("payload_blocks=%d", nBlocks),
		"hashtree="+f.Name())
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", "", err
	}
	ret := f.Name()
	f = nil
	return ret, string(out), nil
}

// appendHash appends the contents of the hash file to the image file.
func appendHash(image, hash string) error {
	content, err := ioutil.ReadFile(hash)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(image, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if file != nil {
			file.Close()
		}
	}()
	if _, err := file.WriteString(string(content)); err != nil {
		return err
	}
	err = file.Close()
	file = nil
	return err
}

// setUpLoop sets up the loopback device for the given image, and returns
// its path.
func setUpLoop(ctx context.Context, image string) (string, error) {
	cmd := testexec.CommandContext(ctx, "losetup", "-f", "--show", image)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// tearDownLoop tears down the loopback device created by setUpLoop.
func tearDownLoop(ctx context.Context, loop string) error {
	return releaseDevice(ctx, []string{"losetup", "-d", loop}, loop)
}

// createVerityDevice sets up the verity device for the given loopback
// device with the name and device table, and returns its path.
func createVerityDevice(ctx context.Context, name, loop, table string) (string, error) {
	devName := "tast_" + name
	devPath := "/dev/mapper/" + devName

	// Clean up stale device file, if exists.
	// Ignore errors, which could be reported in clean state.
	testexec.CommandContext(ctx, "dmsetup", "remove", devPath).Run()

	table = strings.Replace(table, "HASH_DEV", loop, 1)
	table = strings.Replace(table, "ROOT_DEV", loop, 1)
	table = table + " error_behavior=eio"

	cmd := testexec.CommandContext(ctx, "dmsetup", "-r", "create", devName, "--table", table)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	return devPath, nil
}

// removeVerityDevice tears down the verity device created by the
// createVerityDevice.
func removeVerityDevice(ctx context.Context, device string) error {
	return releaseDevice(ctx, []string{"dmsetup", "remove", device}, device)
}

// verifiable walks completely onver the device, and returns any error if
// found.
func verifiable(ctx context.Context, device string) error {
	cmd := testexec.CommandContext(ctx, "dumpe2fs", device)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to run dumpe2fs")
	}

	cmd = testexec.CommandContext(ctx, "dd", "if="+device, "of=/dev/null",
		fmt.Sprintf("bs=%d", blockSize))
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to read device")
	}
	return nil
}

// checkTools returns whether the dm-verity related test can run on the current
// environment.
func checkTools() []error {
	var required = []string{
		"losetup",
		"mkfs.ext3",
		"dmsetup",
		"verity",
		"dd",
		"dumpe2fs",
	}

	var missing []error
	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, err)
		}
	}
	return missing
}

// runCheck sets up the testing environment (specifically sets up device),
// call modify with the created image file, verifies dm-verify behavior
// then tears down the device.
// Returns error if some setup/teardown fails, or the dm-verify behavior is
// different from expect (true on success, false on fail).
func runCheck(ctx context.Context, name string, expect bool, modify func(string) error) error {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	image, err := createImage(ctx, dir, name, nTestBlocks)
	if err != nil {
		return err
	}
	if err := createFileSystem(ctx, image); err != nil {
		return err
	}
	hash, table, err := createHash(ctx, dir, name, image, nTestBlocks)
	if err != nil {
		return err
	}
	// The hash size should be blockSize.
	if fi, err := os.Stat(hash); err != nil {
		return err
	} else if fi.Size() != blockSize {
		return errors.Errorf("unexpected hash file size for %s: got %d; want %d", hash, fi.Size(), blockSize)
	}

	if err := appendHash(image, hash); err != nil {
		return err
	}
	loop, err := setUpLoop(ctx, image)
	if err != nil {
		return err
	}
	defer tearDownLoop(ctx, loop)

	if err = modify(image); err != nil {
		return err
	}

	dev, err := createVerityDevice(ctx, name, loop, table)
	if err != nil {
		return err
	}
	defer removeVerityDevice(ctx, dev)

	err = verifiable(ctx, dev)
	if expect && err != nil {
		return errors.Wrap(err, "unexpected verifiable failure")
	} else if !expect && err == nil {
		return errors.New("unexpected verifiable success")
	}

	return nil
}

func testNoModify(ctx context.Context, s *testing.State) {
	s.Log("Running testNoModify")

	if err := runCheck(ctx, "NoModify", true, func(image string) error { return nil }); err != nil {
		s.Error("NoModify test failed: ", err)
	}
}

func testZeroFill(ctx context.Context, s *testing.State) {
	s.Log("Running testZeroFill")

	// Test 0-filled data for each block.
	for i := 0; i < nTestBlocks; i++ {
		if err := runCheck(ctx, "ZeroFill", false, func(image string) error {
			return testexec.CommandContext(
				ctx, "dd", "if=/dev/zero", "of="+image,
				fmt.Sprintf("bs=%d", blockSize),
				fmt.Sprintf("seek=%d", i), "count=1").Run()
		}); err != nil {
			s.Errorf("ZeroFill test failed at blocks=%d: %v", i, err)
			return
		}
	}
}

func testAFill(ctx context.Context, s *testing.State) {
	s.Log("Running testAFill")

	// Fill hash section of the image by consecutive "A" bytes.
	if err := runCheck(ctx, "AFill", false, func(image string) error {
		f, err := os.OpenFile(image, os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		if _, err := f.WriteAt(bytes.Repeat([]byte("A"), blockSize), nTestBlocks*blockSize); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Error("AFill test failed: ", err)
	}
}

// testBitFlip exercises the bit flip for each block.
func testBitFlip(ctx context.Context, s *testing.State, off int64, mask byte) {
	// Walks nTestBlocks followed by a hash block.
	for i := 0; i < nTestBlocks+1; i++ {
		if err := runCheck(ctx, "BitFlip", false, func(image string) error {
			f, err := os.OpenFile(image, os.O_RDWR, 0666)
			if err != nil {
				return err
			}
			pos := int64(i*blockSize) + off
			buf := make([]byte, 1)
			if _, err = f.ReadAt(buf, pos); err != nil {
				return err
			}
			buf[0] ^= mask
			if _, err = f.WriteAt(buf, pos); err != nil {
				return err
			}
			return nil
		}); err != nil {
			s.Errorf("BitFlip test failed at blocks=%d: %v", i, err)
			return
		}
	}
}

func testFirstBitFlip(ctx context.Context, s *testing.State) {
	s.Log("Running testFirstBitFlip")
	testBitFlip(ctx, s, 0, 0x80)
}

func testMiddleBitFlip(ctx context.Context, s *testing.State) {
	s.Log("Running testMiddleBitFlip")
	testBitFlip(ctx, s, blockSize/2, 0x80)
}

func testLastBitFlip(ctx context.Context, s *testing.State) {
	s.Log("Running testLastBitFlip")
	testBitFlip(ctx, s, blockSize-1, 0x01)
}

// RunTests runs a series of tests related to dm-verity.
func RunTests(ctx context.Context, s *testing.State) {
	if errs := checkTools(); errs != nil {
		for _, err := range errs {
			s.Error("Tool not found: ", err)
		}
		return
	}

	testNoModify(ctx, s)
	testZeroFill(ctx, s)
	testAFill(ctx, s)
	testFirstBitFlip(ctx, s)
	testMiddleBitFlip(ctx, s)
	testLastBitFlip(ctx, s)
}

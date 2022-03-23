// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	blockOpenPrefix = "__block_open_on_autoclear__"
	file1           = "/tmp/" + blockOpenPrefix
	loop1           = "loop101"
	file2           = "/tmp/" + blockOpenPrefix + "trailing"
	loop2           = "loop102"
	file3           = "/tmp/regular"
	loop3           = "loop103"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LoopDeviceBehaviour,
		Desc: "Verifies block_open_on_autoclear behaviour",
		Contacts: []string{
			"bgeffon@gchromium.org",
			"dlunev@chromium.org",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func LoopDeviceBehaviour(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	defer removeBackingFile(cleanupCtx, file1)
	defer removeBackingFile(cleanupCtx, file2)
	defer removeBackingFile(cleanupCtx, file3)
	defer detachLoopDevice(cleanupCtx, loop1)
	defer detachLoopDevice(cleanupCtx, loop2)
	defer detachLoopDevice(cleanupCtx, loop3)

	f1, err := prepareLoopDeviceState(ctx, loop1, file1)
	if err != nil {
		s.Fatal("Can't prepare loop device state: ", err)
	}
	defer f1.Close()

	f2, err := prepareLoopDeviceState(ctx, loop2, file2)
	if err != nil {
		s.Fatal("Can't prepare loop device state: ", err)
	}
	defer f2.Close()

	f3, err := prepareLoopDeviceState(ctx, loop3, file3)
	if err != nil {
		s.Fatal("Can't prepare loop device state: ", err)
	}
	defer f3.Close()

	if canOpenAfterDetach(ctx, loop1) {
		s.Fatal("Device unexpectedly can be opened after detach")
	}

	if canOpenAfterDetach(ctx, loop2) {
		s.Fatal("Device unexpectedly can be opened after detach")
	}

	if !canOpenAfterDetach(ctx, loop3) {
		s.Fatal("Device unexpectedly can't be opened after detach")
	}
}

func createLoopDevice(ctx context.Context, devName, backingFile string) error {
	if _, err := testexec.CommandContext(ctx, "dd", "if=/dev/zero", "of="+backingFile,
		"bs=1M", "count=16").Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "can't create backing file")
	}
	_, err := testexec.CommandContext(ctx, "losetup", devName, backingFile).Output(testexec.DumpLogOnError)
	return err
}

func detachLoopDevice(ctx context.Context, devName string) error {
	_, err := testexec.CommandContext(ctx, "losetup", "-d", "/dev/"+devName).Output(testexec.DumpLogOnError)
	return err
}

func removeBackingFile(ctx context.Context, backingFile string) error {
	return os.Remove(backingFile)
}

func prepareLoopDeviceState(ctx context.Context, devName, backingFile string) (*os.File, error) {
	if err := createLoopDevice(ctx, devName, backingFile); err != nil {
		return nil, errors.Wrap(err, "can't create loop device")
	}

	f, err := os.Open("/dev/" + devName)

	if err != nil {
		return nil, errors.Wrap(err, "can't open loop device after creation")
	}

	if err := detachLoopDevice(ctx, devName); err != nil {
		f.Close()
		return nil, errors.Wrap(err, "can't detach loop device")
	}

	return f, nil
}

func canOpenAfterDetach(ctx context.Context, devName string) bool {
	f, err := os.Open("/dev/" + devName)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

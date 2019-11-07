// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	smallFile       = "/tmp/small_file"
	bigFile         = "/tmp/big_file"
	smallFileDest   = "/home/testuser/small_file"
	bigFileDest     = "/home/testuser/big_file"
	smallUUID       = "fd8a2552-6822-490a-ae67-d43c6ff6e8eb"
	bigUUID         = "ddddffbb-c479-4872-9421-bcf9f1764ed7"
	uuidReplacement = "00000000-0000-0000-0000-000000000000"

	anomalyEventServiceName              = "org.chromium.AnomalyEventService"
	anomalyEventServicePath              = dbus.ObjectPath("/org/chromium/AnomalyEventService")
	anomalyEventServiceInterface         = "org.chromium.AnomalyEventServiceInterface"
	anomalyGuestFileCorruptionSignalName = "GuestFileCorruption"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FsCorruption,
		Desc:         "Check that fs corruption is detected correctly",
		Contacts:     []string{"sidereal@google.com", "mutexlox@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

func createTestFile(ctx context.Context, container *vm.Container, hostPath string, guestPath string, data []byte) error {
	file, err := os.OpenFile(hostPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to create test file")
	}
	if _, err := file.Write(data); err != nil {
		return errors.Wrap(err, "failed to write to test file")
	}
	if err := file.Close(); err != nil {
		return errors.Wrap(err, "failed to close test file")
	}
	if err := container.PushFile(ctx, hostPath, guestPath); err != nil {
		return errors.Wrap(err, "failed to push test file to container")
	}
	return nil
}

func createTestFiles(ctx context.Context, container *vm.Container) error {
	// Small files will get incorporated into the filesystem metadata. Metadata and file data corruption produce different log messages, so we do both.
	if err := createTestFile(ctx, container, smallFile, smallFileDest, []byte(smallUUID)); err != nil {
		return err
	}

	data := make([]byte, 1024*1024)
	copy(data, []byte(bigUUID))
	if err := createTestFile(ctx, container, bigFile, bigFileDest, data); err != nil {
		return err
	}

	return nil
}

func getOffsets(ctx context.Context, filepath string, pattern string) ([]int64, error) {
	var offsets []int64
	cmd := testexec.CommandContext(ctx, "grep", "--fixed-strings", "--byte-offset", "--only-matching", "--binary-files=text", pattern, filepath)

	// Grep returns an output like "12345:pattern\n23456:pattern\n". We parse out the offsets here by splitting on "\n" and then ":". We drop the last line because it will always be empty.
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return make([]int64, 0), errors.Wrap(err, "failed to run grep on disk image")
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[0 : len(lines)-1] {
		firstPart := strings.Split(line, ":")[0]
		i, err := strconv.ParseInt(firstPart, 10, 64)
		if err != nil {
			return make([]int64, 0), errors.Wrapf(err, "got non-int offset %s from line %s of grep output", firstPart, line)
		}
		offsets = append(offsets, i)
	}
	return offsets, nil
}

func writeAtSync(filepath string, b []byte, offset int64) error {
	file, err := os.OpenFile(filepath, os.O_RDWR, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to open VM disk for editing")
	}
	defer file.Close()

	if _, err := file.WriteAt(b, offset); err != nil {
		return errors.Wrapf(err, "failed to write to file at offset %d", offset)
	}
	if err := file.Sync(); err != nil {
		return errors.Wrap(err, "failed to sync write to disk")
	}

	return nil
}

func waitForSignal(ctx context.Context, signalWatcher *dbusutil.SignalWatcher) (*dbus.Signal, error) {
	select {
	case signal := <-signalWatcher.Signals:
		return signal, nil
	case <-ctx.Done():
		return nil, errors.New("Context deadline expired")
	}
}

func testOverwriteAtOffset(ctx context.Context, offset int64, container *vm.Container, backupPath string, outDir string) error {
	match := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      anomalyEventServicePath,
		Interface: anomalyEventServiceInterface,
		Member:    anomalyGuestFileCorruptionSignalName,
	}
	signalWatcher, err := dbusutil.NewSignalWatcherForSystemBus(ctx, match)
	if err != nil {
		return errors.Wrap(err, "failed to listed for DBus signals")
	}
	defer signalWatcher.Close(ctx)

	// Defer restoring from backup before attempting the write, because if the write fails the disk is in an unknown state.
	cmd := testexec.CommandContext(ctx, "cp", "--sparse=always", "--backup=off", "--preserve=all", backupPath, container.VM.DiskPath)
	defer cmd.Run(testexec.DumpLogOnError)

	// Make edit to disk at this offset.
	testing.ContextLog(ctx, "Making changes at offset ", offset)
	if err := writeAtSync(container.VM.DiskPath, []byte(uuidReplacement), offset); err != nil {
		return errors.Wrap(err, "failed to make disk edit")
	}

	testing.ContextLog(ctx, "Restarting VM")
	if err := vm.RestartDefaultVMContainer(ctx, outDir, container); err != nil {
		return errors.Wrap(err, "failed to restart VM")
	}
	defer container.VM.Stop(ctx)

	// Filesystem corruption doesn't get detected until some process tries to read from the file. For metadata, this usually happens during container startup, but we read from both files just to be sure.
	testing.ContextLog(ctx, "Attempting to read corrupted files")
	// Error is expected, ignore it.
	_ = container.Command(ctx, "cat", bigFileDest).Run()
	_ = container.Command(ctx, "cat", smallFileDest).Run()

	testing.ContextLog(ctx, "Waiting for signal from anomaly_detector")
	ctx, _ = context.WithTimeout(ctx, 5*time.Second)
	if _, err := waitForSignal(ctx, signalWatcher); err != nil {
		return errors.Wrap(err, "didn't get expected DBus signal")
	}

	return nil
}

func testOffsetList(ctx context.Context, offsets []int64, container *vm.Container, backupPath string, outDir string) []error {
	errs := make([]error, 0, len(offsets))
	for _, offset := range offsets {
		if err := testOverwriteAtOffset(ctx, offset, container, backupPath, outDir); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == len(offsets) {
		return errs
	}
	return nil
}

// FsCorruption sets up the VM and then introduces corruption into its disk to check that this is detected correctly. We do not use the crostini preconditions to avoid disrupting other crostini tests, instead we invoke the Prepare method directly.
func FsCorruption(ctx context.Context, s *testing.State) {
	pre := crostini.StartedByDownload().(*crostini.PreImpl)
	data := pre.Prepare(ctx, s).(crostini.PreData)
	defer pre.Close(ctx, s)

	s.Log("Writing test file to container")
	if err := createTestFiles(ctx, data.Container); err != nil {
		s.Fatal("Failed to create test files: ", err)
	}

	// Stop the VM so it isn't running while we edit it's disk.
	s.Log("Stopping VM")
	if err := data.Container.VM.Stop(ctx); err != nil {
		s.Fatal("Failed to stop VM: ", err)
	}
	// Restart everything before finishing so the precondition will close correctly.
	defer vm.RestartDefaultVMContainer(ctx, s.OutDir(), data.Container)

	s.Log("Searching for pattern in disk image")
	bigOffsets, err := getOffsets(ctx, data.Container.VM.DiskPath, bigUUID)
	if err != nil {
		s.Fatal("Failed to get file offsets: ", err)
	}

	smallOffsets, err := getOffsets(ctx, data.Container.VM.DiskPath, smallUUID)
	if err != nil {
		s.Fatal("Failed to get file offsets: ", err)
	}

	// BTRFS filesystems are modified on every mount, so we make a backup here of the disk so we can start each corruption from a known state.
	s.Log("Backing up the current disk image")
	backupPath := data.Container.VM.DiskPath + ".bak"
	cmd := testexec.CommandContext(ctx, "cp", "--sparse=always", "--backup=off", "--preserve=all", data.Container.VM.DiskPath, backupPath)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		testexec.CommandContext(ctx, "rm", "--force", backupPath).Run()
		s.Fatal("Failed to back up VM disk for editing: ", err)
	}

	// Always restore the backup disk before ending the test.
	cmd = testexec.CommandContext(ctx, "mv", "--force", backupPath, data.Container.VM.DiskPath)
	defer cmd.Run(testexec.DumpLogOnError)

	// Data can be stored redundantly in the file system. If this is done, then only one of the copies will actually be read and cause an error. Therefore, we only require one error per file while we try to damage each copy of the data in turn.
	if errs := testOffsetList(ctx, bigOffsets, data.Container, backupPath, s.OutDir()); errs != nil {
		s.Fatal("Didn't get an error signal for any overwrite point for big file: ", errs)
	}
	if errs := testOffsetList(ctx, smallOffsets, data.Container, backupPath, s.OutDir()); errs != nil {
		s.Fatal("Didn't get an error signal for any overwrite point for small file: ", errs)
	}
}

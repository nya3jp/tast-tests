// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/crash"
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
	fsCorruptionHistogram                = "Crostini.FilesystemCorruption"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FsCorruption,
		Desc: "Check that fs corruption is detected correctly",
		Contacts: []string{
			// Crostini
			"sidereal@google.com",
			"cros-containers-dev@google.com",
			// Telemetry
			"mutexlox@google.com",
			"cros-telemetry@google.com",
		},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"keepState"},
		Timeout:      10 * time.Minute,
		Pre:          crostini.StartedByDownloadBuster(),
	})
}

func createTestFile(ctx context.Context, container *vm.Container, hostPath, guestPath string, data []byte) error {
	if err := ioutil.WriteFile(hostPath, data, 0755); err != nil {
		return errors.Wrap(err, "failed to write to test file")
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

func getOffsets(ctx context.Context, filepath string, smallPattern, bigPattern []byte) ([]int64, []int64, error) {
	var smallOffsets, bigOffsets []int64

	// Buffer size = 1MiB + a small amount of overlap in case one of the UUIDs is split across a boundary
	buffer := make([]byte, 1024*1024+1024)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open VM disk for reading")
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to stat VM disk")
	}
	size := stat.Size()

	var i int64
	for i = 0; i < size; i += 1024 * 1024 {
		read, err := file.ReadAt(buffer, i)
		if err != nil && err != io.EOF {
			return nil, nil, errors.Wrapf(err, "failed to read from VM disk at offset %d", i)
		} else if err == io.EOF {
			// We read less then the full buffer because we hit EOF, so truncate the buffer
			buffer = buffer[:read]
		} else if read < len(buffer) {
			return nil, nil, errors.Errorf("failed to read a full buffer from VM disk at offset %d, got %d bytes, expected %d", i, read, len(buffer))
		}

		scan := buffer
		accum := i
		for true {
			idx := bytes.Index(scan, smallPattern)
			if idx == -1 {
				break
			}

			newOffset := accum + int64(idx)
			if len(smallOffsets) == 0 || smallOffsets[len(smallOffsets)-1] != newOffset {
				smallOffsets = append(smallOffsets, newOffset)
			}

			scan = scan[idx+1:]
			accum += int64(idx) + 1
		}

		scan = buffer
		accum = i
		for true {
			idx := bytes.Index(scan, bigPattern)
			if idx == -1 {
				break
			}

			newOffset := accum + int64(idx)
			if len(bigOffsets) == 0 || bigOffsets[len(bigOffsets)-1] != newOffset {
				bigOffsets = append(bigOffsets, newOffset)
			}

			scan = scan[idx+1:]
			accum += int64(idx) + 1
		}
	}

	return smallOffsets, bigOffsets, nil
}

func writeAtSync(filepath string, b []byte, offsets []int64) error {
	file, err := os.OpenFile(filepath, os.O_RDWR, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to open VM disk for editing")
	}
	defer file.Close()

	for _, offset := range offsets {
		if _, err := file.WriteAt(b, offset); err != nil {
			return errors.Wrapf(err, "failed to write to file at offset %d", offset)
		}
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

func checkHistogram(ctx context.Context, tconn *chrome.TestConn, baseline int64) (int64, error) {
	hist, err := metrics.GetHistogram(ctx, tconn, fsCorruptionHistogram)
	if err != nil {
		return 0, err
	}
	if hist.Sum <= baseline {
		return hist.Sum, errors.Errorf("expected total of more then %v histogram values, got %v", baseline, hist.Sum)
	}
	return hist.Sum, nil
}

// testOverwriteAtOffsets overwrites the VM disk that stores
// container at the locations in offsets with uuidReplacement. It
// then restarts the VM and container and checks that the filesystem
// corruption is detected. Finally, it stops the VM and restores the VM
// disk from backupPath. outDir is passed to
// vm.RestartDefaultVMContainer and may be used to store logs from
// container startup on failure.
func testOverwriteAtOffsets(ctx context.Context, offsets []int64, container *vm.Container, diskPath, backupPath, outDir string) error {
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
	cmd := testexec.CommandContext(ctx, "cp", "--sparse=always", "--backup=off", "--preserve=all", backupPath, diskPath)
	defer cmd.Run(testexec.DumpLogOnError)

	// Make edit to disk at these offsets.
	testing.ContextLog(ctx, "Making changes at offsets ", offsets)
	if err := writeAtSync(diskPath, []byte(uuidReplacement), offsets); err != nil {
		return errors.Wrap(err, "failed to make disk edit")
	}

	testing.ContextLog(ctx, "Restarting VM")
	// Discard the error, as this may fail due to corruption.
	_ = vm.RestartDefaultVMContainer(ctx, outDir, container)
	defer container.VM.Stop(ctx)

	// Filesystem corruption doesn't get detected until some process tries to read from the corrupted location. For metadata, this usually happens during container startup, but we read from both files just to be sure.
	testing.ContextLog(ctx, "Attempting to read corrupted files")
	// Error is expected, ignore it.
	_ = container.Command(ctx, "cat", bigFileDest).Run()
	_ = container.Command(ctx, "cat", smallFileDest).Run()

	testing.ContextLog(ctx, "Waiting for signal from anomaly_detector")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := waitForSignal(ctx, signalWatcher); err != nil {
		return errors.Wrap(err, "didn't get expected DBus signal")
	}

	return nil
}

// FsCorruption sets up the VM and then introduces corruption into its disk to check that this is detected correctly.
func FsCorruption(ctx context.Context, s *testing.State) {
	data := s.PreValue().(crostini.PreData)
	cr := data.Chrome
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	s.Log("Writing test file to container")
	if err := createTestFiles(ctx, data.Container); err != nil {
		s.Fatal("Failed to create test files: ", err)
	}

	disk, err := data.Container.VM.Concierge.GetVMDiskInfo(ctx, vm.DefaultVMName)
	if err != nil {
		s.Fatal("Failed to get VM disk info: ", err)
	}

	// Stop the VM so it isn't running while we edit its disk.
	s.Log("Stopping VM")
	if err := data.Container.VM.Stop(ctx); err != nil {
		s.Fatal("Failed to stop VM: ", err)
	}
	// Restart everything before finishing so the precondition will be in a good state.
	defer vm.RestartDefaultVMContainer(ctx, s.OutDir(), data.Container)

	s.Log("Searching for pattern in disk image")
	smallOffsets, bigOffsets, err := getOffsets(ctx, disk.GetPath(), []byte(smallUUID), []byte(bigUUID))
	if err != nil || len(smallOffsets) == 0 || len(bigOffsets) == 0 {
		s.Fatal("Failed to get file offsets: ", err)
	}

	// BTRFS filesystems are modified on every mount, so we make a backup here of the disk so we can start each corruption from a known state.
	s.Log("Backing up the current disk image")
	backupPath := disk.GetPath() + ".bak"
	cmd := testexec.CommandContext(ctx, "cp", "--sparse=always", "--backup=off", "--preserve=all", disk.GetPath(), backupPath)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		testexec.CommandContext(ctx, "rm", "--force", backupPath).Run()
		s.Fatal("Failed to back up VM disk for editing: ", err)
	}

	// Always restore the backup disk before ending the test.
	cmd = testexec.CommandContext(ctx, "mv", "--force", backupPath, disk.GetPath())
	defer cmd.Run(testexec.DumpLogOnError)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	histogramCount, err := checkHistogram(ctx, tconn, -1)
	if err != nil {
		s.Fatal("Failed to get baseline for histogram: ", err)
	}

	if err := testOverwriteAtOffsets(ctx, bigOffsets, data.Container, disk.GetPath(), backupPath, s.OutDir()); err != nil {
		s.Fatal("Didn't get an error signal for big file: ", err)
	}
	histogramCount, err = checkHistogram(ctx, tconn, histogramCount)
	if err != nil {
		s.Fatal("Failed to check histogram: ", err)
	}

	if err := testOverwriteAtOffsets(ctx, smallOffsets, data.Container, disk.GetPath(), backupPath, s.OutDir()); err != nil {
		s.Fatal("Didn't get an error signal for small file: ", err)
	}
	if _, err := checkHistogram(ctx, tconn, histogramCount); err != nil {
		s.Fatal("Failed to check histogram: ", err)
	}
}

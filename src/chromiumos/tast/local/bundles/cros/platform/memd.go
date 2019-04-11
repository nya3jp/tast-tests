// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	metrics_event "chromiumos/system_api/metrics_event_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Memd,
		Desc:         "Checks that memd works",
		Contacts:     []string{"semenzato@chromium.org"},
		SoftwareDeps: []string{"memd"},
	})
}

func readAsInt(filename string) (int, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	str := strings.TrimSpace(string(bytes))
	return strconv.Atoi(str)
}

// Emits a D-Bus signal identical to the one sent by Chrome to notify memd that
// a tab discard has occurred.
func emitTabDiscardSignal() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	// Create metrics event instance.
	event := &metrics_event.Event{
		Type:      metrics_event.Event_TAB_DISCARD,
		Timestamp: 12345,
	}
	bytes, err := proto.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "emit tab discard: marshal failed")
	}
	// Emit signal with byte array as payload.
	return conn.Emit("/",
		"org.chromium.MetricsEventServiceInterface.ChromeEvent", bytes)
}

// Logs a syslog entry which makes the anomaly detector emit an OOM-kill D-Bus
// signal.
func logFakeOOMKill(ctx context.Context) error {
	// Take advantage of the fact that the anomaly_detector scanner is not
	// strict and will ignore the first part of the line.
	fakeMessage := "kernel: [ 8996.861500] Out of memory: Kill process"
	cmd := testexec.CommandContext(ctx, "logger", fakeMessage)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "logger failed")
	}
	return nil
}

func checkClipFiles(s *testing.State, pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		s.Fatalf("Cannot glob %v: %v", pattern, err)
	}
	if len(files) == 0 {
		return errors.New("no clip files found")
	}
	// It's unlikely, but not impossible, that unforeseen events triggered
	// the creation of more than one clip file.  Thus we are not picky
	// about the number of files, but we need to ensure that at least one
	// DISCRD and one KEROOM events were generated.
	var discrdFound, keroomFound bool
	for _, file := range files {
		clipBytes, err := ioutil.ReadFile(file)
		if err != nil {
			s.Fatalf("Cannot read content of %v: %v", file, err)
		}
		clipString := string(clipBytes)
		if strings.Contains(clipString, "discrd") {
			discrdFound = true
		}
		if strings.Contains(clipString, "keroom") {
			keroomFound = true
		}
	}
	if keroomFound && discrdFound {
		return nil
	}
	if keroomFound {
		return errors.New("discard event is missing")
	}
	if discrdFound {
		return errors.New("kernel OOM event is missing")
	}
	return errors.New("no events found")
}

func Memd(ctx context.Context, s *testing.State) {
	const (
		// This value should be the same as LOW_MEM_DANGER_THRESHOLD_MB
		// in memd/src/main.rs.
		dangerThreshold  = 600
		lowMemDirPath    = "/sys/kernel/mm/chromeos-low_mem/"
		availablePath    = lowMemDirPath + "available"
		marginPath       = lowMemDirPath + "margin"
		clipFilesPattern = "/var/log/memd/memd.clip*.log"
		memdJob          = "memd"
	)

	_, _, memdPID, err := upstart.JobStatus(ctx, memdJob)
	if err != nil {
		s.Fatal("Could not get memd job status: ", err)
	}
	if memdPID == 0 {
		s.Fatal("memd is not running")
	}

	originalMargin, err := readAsInt(marginPath)
	if err != nil {
		s.Fatalf("Cannot read %v: %v", marginPath, err)
	}

	// Set up actions to be taken on exit (either normal exit or fatal
	// error) to restore the original state, which is: memd must be
	// running, and the low-mem margin must have its original value.  This
	// requires reading originalMargin first.
	defer func() {
		// Restore the original margin.  (We don't know if it has been
		// changed yet, but it doesn't matter.)
		if err := ioutil.WriteFile(marginPath,
			[]byte(strconv.Itoa(originalMargin)), 0644); err != nil {
			s.Errorf("Cannot write %v: %v", marginPath, err)
		}
		// Restart memd to pick up the original margin.  Note that
		// upstart.Restart is not the same as 'initctl restart' and
		// tolerates a stopped job, which may be the case here.
		if err = upstart.RestartJob(ctx, memdJob); err != nil {
			s.Error("Cannot restart memd: ", err)
		}
	}()

	// Remove any clip files from /var/log/memd.
	files, err := filepath.Glob(clipFilesPattern)
	if err != nil {
		s.Fatalf("Cannot list %v: %v", clipFilesPattern, err)
	}
	for _, file := range files {
		if err = os.Remove(file); err != nil {
			s.Fatalf("Cannot remove %v: %v", file, err)
		}
	}

	available, err := readAsInt(availablePath)
	if err != nil {
		s.Fatalf("Cannot read %v: %v", availablePath, err)
	}

	// Raise notification margin so that memd starts running in fast poll
	// mode.  Add 100 to the minimum required value because available
	// memory may change.  Try multiple times.
	success := false
	var margin int
	for triesCount := 0; triesCount < 3; triesCount++ {
		margin = available - dangerThreshold + 100
		if err := ioutil.WriteFile(marginPath,
			[]byte(strconv.Itoa(margin)), 0644); err != nil {
			s.Fatalf("Cannot write %v: %v", marginPath, err)
		}
		available, err = readAsInt(availablePath)
		if err != nil {
			s.Fatalf("Cannot read %v: %v", availablePath, err)
		}
		if margin+dangerThreshold > available {
			success = true
			break
		}
	}
	if !success {
		s.Fatalf("Cannot adjust margin: available = %v, margin = %v, "+
			"dangerThreshold = %v (MB)", available, margin,
			dangerThreshold)
	}

	// Restart memd to pick up the new margin.
	if err = upstart.RestartJob(ctx, memdJob); err != nil {
		s.Fatal("Cannot restart memd: ", err)
	}

	// Wait some time to ensure memd goes into fast-poll mode and starts
	// filling the ring buffer.  The wait must be longer than
	// SLOW_POLL_PERIOD_DURATION in memd/src/main.rs.
	s.Log("Waiting for memd to enter fast-poll mode")
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed waiting for memd: ", err)
	}

	// Send a fake tab-discard notification.
	if err = emitTabDiscardSignal(); err != nil {
		s.Fatal("Cannot send fake tab discard signal: ", err)
	}
	// Produce an OOM-kill notification by logging a line that tickles the
	// anomaly detector.
	if err = logFakeOOMKill(ctx); err != nil {
		s.Fatal("Cannot log fake oom event: ", err)
	}

	// Check that one or more clip files have been generated with the
	// requested events.  Normally this will take 5 seconds, but it could
	// take less or more if other events (which may be added in the future)
	// trigger a clip collection.  Worst case this will take 10 seconds.
	// Add an extra 5 seconds before timing out.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return checkClipFiles(s, clipFilesPattern)
	}, &testing.PollOptions{
		Timeout:  (10 + 5) * time.Second,
		Interval: 100 * time.Millisecond,
	})
	if err != nil {
		s.Error("Failed after waiting for memd output: ", err)
	}
}

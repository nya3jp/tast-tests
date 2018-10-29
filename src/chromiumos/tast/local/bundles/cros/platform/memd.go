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
		Func: Memd,
		Desc: "Checks that memd works",
		Attr: []string{"informational"},
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
		return errors.Wrap(err, "emitting tab discard signal")
	}
	// Emit signal with byte array as payload.
	return conn.Emit("/",
		"org.chromium.MetricsEventServiceInterface.ChromeEvent", bytes)
}

// Logs a syslog entry which makes the anomaly collector emit an OOM-kill D-Bus
// signal.
func logFakeOOMKill(ctx context.Context) error {
	// Take advantage of the fact that the anomaly_collector scanner is not
	// strict and will ignore the first part of the line.
	fakeMessage := "kernel: [ 8996.861500] Out of memory: Kill process"
	cmd := testexec.CommandContext(ctx, "logger", fakeMessage)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "logger failed")
	}
	return nil
}

func checkClipFiles(ctx context.Context, s *testing.State, clipFilesPattern string) (int, bool, bool, error) {
	files, err := filepath.Glob(clipFilesPattern)
	if err != nil {
		s.Fatalf("Cannot glob %v: %v", clipFilesPattern, err)
	}
	filesCount := len(files)
	if filesCount == 0 {
		return 0, false, false, errors.New("no clip files found")
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
		err = nil
	} else {
		err = errors.New("events are missing")
	}
	return filesCount, discrdFound, keroomFound, err
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
	)

	_, _, memdPID, err := upstart.JobStatus(ctx, "memd")
	if err != nil {
		s.Fatal("could not get memd job status: ", err)
	}
	if memdPID == 0 {
		s.Fatal("memd is not running")
	}

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

	// Wait some time to ensure memd notices the change and starts filling
	// the ring buffer.  The wait must be longer than
	// SLOW_POLL_PERIOD_DURATION in memd/src/main.rs.
	time.Sleep(3 * time.Second)

	// Send a fake tab-discard notification.
	if err = emitTabDiscardSignal(); err != nil {
		s.Fatal("Cannot send fake tab discard signal: ", err)
	}
	// Produce an OOM-kill notification by logging a line that tickles the
	// anomaly collector.
	if err = logFakeOOMKill(ctx); err != nil {
		s.Fatal("Cannot log fake oom event: ", err)
	}

	// We must wait at least 5 seconds to ensure that the clip file from
	// these events is dumped (and that's how long it will take most
	// times).  We cannot stop as soon as we notice a file being dumped
	// because it may be generated by other events.  (Unlikely now, but
	// could easily happen in the future as we add events of interest).
	time.Sleep(5100 * time.Millisecond)

	// Now ensure that at least one clip file has been generated, with a
	// generous timeout.  Note that the closure in Poll sets various
	// variables.
	var filesCount int
	var discrdFound, keroomFound bool
	err = testing.Poll(ctx, func(ctx context.Context) error {
		filesCount, discrdFound, keroomFound, err =
			checkClipFiles(ctx, s, clipFilesPattern)
		return err
	}, &testing.PollOptions{
		Timeout:  6 * time.Second,
		Interval: 100 * time.Millisecond,
	})
	if filesCount == 0 {
		s.Fatal("No clip files were produced")
	}
	if !discrdFound {
		s.Error("No discrd event found")
	}
	if !keroomFound {
		s.Error("No keroom event found")
	}
}

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
	"github.com/pkg/errors"

	metrics_event "chromiumos/system_api/metrics_event_proto"
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
		return errors.Wrapf(err, "emitting tab discard signal")
	}
	// Emit signal with byte array as payload.
	return conn.Emit("/",
		"org.chromium.MetricsEventServiceInterface.ChromeEvent", bytes)
}

// Logs a syslog entry which makes the anomaly collector emit an OOM-kill D-Bus
// signal.
func logFakeOomKill(ctx context.Context) error {
	// Take advantage of the fact that the anomaly_collector scanner is not
	// strict and will ignore the first part of the line.
	fakeMessage := "kernel: [ 8996.861500] Out of memory: Kill process"
	cmd := testexec.CommandContext(ctx, "logger", fakeMessage)
	_, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrapf(err, "logger failed")
	}
	return nil
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
		s.Fatal("Memd initctl status: ", err)
	}
	if memdPID == 0 {
		s.Fatal("Memd is not running")
	}

	// Remove any clip files from /var/log/memd.
	files, err := filepath.Glob(clipFilesPattern)
	if err != nil {
		s.Fatal("Cannot list /var/log/memd: ", err)
	}
	for _, file := range files {
		err = os.Remove(file)
		if err != nil {
			s.Fatalf("Cannot remove %v: %v", file, err)
		}
	}

	available, err := readAsInt(availablePath)
	if err != nil {
		s.Fatal("cannot read 'available' sysfs: ", err)
	}
	margin, err := readAsInt(marginPath)
	if err != nil {
		s.Fatal("cannot read 'margin' sysfs: ", err)
	}

	// Raise notification margin so that memd starts running in fast poll
	// mode.  Add 100 to the minimum required value because available
	// memory may change.  Try multiple times.
	success := false
	for triesCount := 0; triesCount < 3; triesCount++ {
		margin = available - dangerThreshold + 100
		if err := ioutil.WriteFile(marginPath,
			[]byte(strconv.Itoa(margin)), 0644); err != nil {
			s.Fatalf("%v: cannot write", marginPath)
		}
		available, err = readAsInt(availablePath)
		if err != nil {
			s.Fatalf("cannot read %v: %v", availablePath, err)
		}
		if margin+dangerThreshold > available {
			success = true
			break
		}
	}
	if !success {
		s.Fatalf("available RAM grows too fast, currently %v MB",
			available)
	}

	// Wait a couple of seconds to ensure memd notices the change and
	// starts filling the ring buffer.
	time.Sleep(2 * time.Second)

	// Send a fake tab-discard notification.
	if err = emitTabDiscardSignal(); err != nil {
		s.Fatal("cannot send fake tab discard signal: ", err)
	}
	// Produce an OOM-kill notification by logging a line that tickles the
	// anomaly collector.
	if err = logFakeOomKill(ctx); err != nil {
		s.Fatal("cannot log fake oom event: ", err)
	}

	// It should take 5 seconds for a clip file to be dumped.  Wait until
	// that happens, or the timeout hits.  The closure in Poll sets the
	// |files| variable so it's ready to use.
	testing.Poll(ctx, func(ctx context.Context) error {
		files, err = filepath.Glob(clipFilesPattern)
		if err != nil {
			s.Fatalf("cannot glob %v: %v", clipFilesPattern, err)
		}
		if len(files) == 0 {
			return errors.New("keep polling")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  (5 + 10) * time.Second,
		Interval: time.Second,
	})

	if len(files) == 0 {
		s.Fatal("memd did not create a clip file")
	}

	// It's unlikely, but not impossible, that other events triggered the
	// creation of more than one clip file.  We only need to ensure that at
	// least one DISCRD and one KEROOM events were generated.
	var discrdFound, keroomFound bool
	for _, file := range files {
		clipBytes, err := ioutil.ReadFile(file)
		if err != nil {
			s.Fatalf("cannot read content of %v: %v", file, err)
		}
		clipString := string(clipBytes)
		if strings.Contains(clipString, "discrd") {
			discrdFound = true
		}
		if strings.Contains(clipString, "keroom") {
			keroomFound = true
		}
	}
	if !discrdFound {
		s.Error("no keroom event found")
	}
	if !keroomFound {
		s.Error("no discrd event found")
	}
}

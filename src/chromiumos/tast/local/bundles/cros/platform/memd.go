// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"log/syslog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"chromiumos/system_api/metrics_event_proto"
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

func writeInt(filename string, value int) error {
	str := strconv.Itoa(value)
	return ioutil.WriteFile(filename, []byte(str), 0644)
}

func fakeTabDiscardSignal() error {
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
		return errors.Wrapf(err, "fake tab discard")
	}
	// Emit signal with byte array as payload.
	return conn.Emit("/",
		"org.chromium.MetricsEventServiceInterface.ChromeEvent", bytes)
}

func fakeOomKillSignal() error {
	logwriter, err := syslog.New(syslog.LOG_NOTICE, "memd-test")
	if err != nil {
		return errors.Wrap(err, "oom kill")
	}
	// Take advantage of the fact that the anomaly_collector scanner is not
	// strict and will ignore the first part of the line.
	fakeMessage := "kernel: [ 8996.861500] Out of memory: Kill process"
	logwriter.Write([]byte(fakeMessage))
	return nil
}

func sleep(ctx context.Context, sleepTime time.Duration) {
	testing.Poll(ctx, func(ctx context.Context) error {
		return errors.New("keep polling")
	}, &testing.PollOptions{Timeout: sleepTime, Interval: sleepTime})
}

func Memd(ctx context.Context, s *testing.State) {
	const (
		// This value should be the same as LOW_MEM_DANGER_THRESHOLD_MB
		// in memd/src/main.rs.
		dangerThreshold = 600
		lowMemDirName   = "/sys/kernel/mm/chromeos-low_mem/"
		availableName   = lowMemDirName + "available"
		marginName      = lowMemDirName + "margin"
	)

	_, _, memdPid, err := upstart.JobStatus(ctx, "memd")
	if err != nil {
		s.Fatal("memd initctl status: ", err)
	}
	if memdPid == 0 {
		s.Fatal("memd is not running")
	}

	// Remove any clip files from /var/log/memd.
	files, err := filepath.Glob("/var/log/memd/memd.clip*.log")
	if err != nil {
		s.Fatal("cannot list /var/log/memd: ", err)
	}
	for _, file := range files {
		err = os.Remove(file)
		if err != nil {
			s.Fatal("cannot remove ", file, ": ", err)
		}
	}

	available, err := readAsInt(availableName)
	if err != nil {
		s.Fatal("cannot read 'available' sysfs:", err)
	}
	margin, err := readAsInt(marginName)
	if err != nil {
		s.Fatal("cannot read 'margin' sysfs:", err)
	}

	// Raise notification margin so that memd starts running in fast poll
	// mode.  Add 100 to the minimum required value because available
	// memory may change.
	success := false
	triesLimit := 3
	for triesCount := 0; triesCount < triesLimit; triesCount++ {
		margin = available - dangerThreshold + 100
		writeInt(marginName, margin)
		available, err = readAsInt(availableName)
		if err != nil {
			s.Fatal("failed to read 'available' sysfs", err)
		}
		if margin+dangerThreshold > available {
			success = true
			break
		}
	}
	if !success {
		s.Fatal("available RAM grows too fast, currently ", available)
	}

	// Wait a couple of seconds to ensure memd notices the change and
	// starts filling the ring buffer.
	sleep(ctx, 2 * time.Second)

	// Send a fake tab-discard notification.
	err = fakeTabDiscardSignal()
	if err != nil {
		s.Fatal("cannot send fake tab discard signal: ", err)
	}
	// Produce a fake OOM-kill notification.
	err = fakeOomKillSignal()
	if err != nil {
		s.Fatal("cannot send fake oom signal: ", err)
	}

	// Wait at least 5 seconds for a clip file to be dumped.
	sleep(ctx, (5 + 1) * time.Second)

	// Verify that the clip file has been dumped.  Don't check for Glob
	// errors again.
	files, _ = filepath.Glob("/var/log/memd/memd.clip*.log")
	if len(files) == 0 {
		s.Fatal("memd did not create a clip file")
	}

	// It's unlikely, but not impossible, that other events triggered the
	// creation of more than one clip file.  We only need to ensure that at
	// least one DISCRD and one KEROOM events were generated.
	discrdFound := false
	keroomFound := false
	for _, file := range files {
		clipBytes, err := ioutil.ReadFile(file)
		if err != nil {
			s.Fatal("cannot read content of ", file, ": ", err)
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

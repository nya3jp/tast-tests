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

	"chromiumos/system_api/metrics_event_proto"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Memd,
		Desc: "Checks that memd works",
		Attr: []string{"informational"},
	})
}

// This value should be the same as LOW_MEM_DANGER_THRESHOLD_MB in
// memd/src/main.rs.
const Threshold = 600

const LowMemDirName = "/sys/kernel/mm/chromeos-low_mem/"
const Available = LowMemDirName + "available"
const Margin = LowMemDirName + "margin"

func ReadAsInt(filename string, s *testing.State) int {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		s.Fatal("cannot read ", filename, ": ", err)
	}
	str := strings.TrimSpace(string(bytes))
	x, err := strconv.Atoi(str)
	if err != nil {
		s.Fatal("cannot convert '", str, "' to int: ", err)
	}
	return x
}

func WriteInt(filename string, value int, s *testing.State) {
	str := strconv.Itoa(value)
	if err := ioutil.WriteFile(filename, []byte(str), 0644); err != nil {
		s.Fatal("cannot write to ", filename, ": ", err)
	}
}

func FakeTabDiscardSignal(s *testing.State) {
	conn, err := dbus.SystemBus()
	if err != nil {
		s.Fatal(os.Stderr, "connect to system bus:", err)
	}
	// Create metrics event instance.
	event := new(metrics_event.Event)
	event.Type = metrics_event.Event_TAB_DISCARD
	event.Timestamp = 12345
	// Convert it to byte array, ignoring impossible errors.
	bytes, _ := proto.Marshal(event)
	// Emit signal with byte array as payload.
	err = conn.Emit("/",
		"org.chromium.MetricsEventServiceInterface.ChromeEvent", bytes)
	if err != nil {
		s.Fatal(os.Stderr, "emit signal:", err)
	}
}

func FakeOomKillSignal(s *testing.State) {
	logwriter, err := syslog.New(syslog.LOG_NOTICE, "memd-test")
	if err != nil {
		s.Fatal(os.Stderr, "cannot write to syslog:", err)
	}
	// Take advantage of the fact that the anomaly_collector scanner is not
	// strict and will ignore the first part of the line.
	logwriter.Warning("kernel: [ 8996.861500] Out of memory: Kill process")
}


func Memd(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "status", "memd")
	bytes, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("command 'status memd' failed: ", err)
	}
	if !strings.Contains(string(bytes), "memd start/running") {
		s.Fatal("unexpected memd status output: ", string(bytes))
	}

	// Remove any clip files from /var/log/memd.
	files, err := filepath.Glob("/var/log/memd/memd.clip*.log")
        if err != nil {
		s.Fatal("cannot list /var/log/memd: ", err)
	}
        for _, file := range(files) {
		err = os.Remove(file)
		if err != nil {
			s.Fatal("cannot remove ", file, ": ", err)
		}
	}

	available := ReadAsInt(Available, s)
	margin := ReadAsInt(Margin, s)

	// Raise notification margin so that memd starts running in fast poll
	// mode.  Add 100 to the minimum required value because available
	// memory may change.
	tries_count := 0
	tries_limit := 3
	for margin + Threshold < available && tries_count < tries_limit {
		margin = available - Threshold + 100
		WriteInt(Margin, margin, s)
		available = ReadAsInt(Available, s)
		tries_count += 1
	}
	if tries_count == tries_limit {
		s.Fatal("available RAM changes too much, now ", available)
	}

	// Wait a couple of seconds to ensure memd notices the change and
	// starts filling the ring buffer.
	time.Sleep(2 * time.Second)

	// Send a fake tab-discard notification.
	FakeTabDiscardSignal(s)
	// Produce a fake OOM-kill notification.
	FakeOomKillSignal(s)

	// Wait at least 5 seconds for a clip file to be dumped.
	time.Sleep((5 + 1) * time.Second)

	// Verify that the clip file has been dumped.  Don't check for Glob
	// errors again.
	files, _ = filepath.Glob("/var/log/memd/memd.clip*.log")
	if len(files) == 0 {
		s.Fatal("memd did not create a clip file")
	}
	// Look for the two events in the first clip file.
	clip_bytes, err := ioutil.ReadFile(files[0])
	if err != nil {
		s.Fatal("cannot read content of ", files[0], ": ", err)
	}
	clip_string := string(clip_bytes)
	if !strings.Contains(clip_string, "discrd") {
		s.Error("no discrd event found")
	}
	if !strings.Contains(clip_string, "keroom") {
		s.Error("no keroom event found")
	}
}

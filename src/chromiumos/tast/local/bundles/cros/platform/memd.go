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

	"chromiumos/system_api/metrics_event_proto"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Memd,
		Desc:         "Checks that memd works",
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
	x, err2 := strconv.Atoi(str)
	if err2 != nil {
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

func FakeOomKillSignal(s *testing.State) {
	conn, err := dbus.SessionBus()
	if err != nil {
		s.Fatal(os.Stderr, "Failed to connect to session bus:", err)
	}
	// Create metrics event instance.
	event := new(metrics_event.Event)
	event.Type = metrics_event.Event_OOM_KILL_KERNEL
	event.Timestamp = 0
	// Convert it to byte array, ignoring impossible errors.
	bytes, _ := proto.Marshal(event)
	// Emit signal with byte array as payload.
	conn.Emit("/org/chromium/AnomalyEventService",
		"org.chromium.AnomalyEventServiceInterface.AnomalyEvent", bytes)
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
	files, err2 := filepath.Glob("/var/log/memd/memd.clip*.log")
        if err2 != nil {
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
		s.Fatal("wild swings of available RAM, currently ", available)
	}

	// Wait 2 seconds to ensure memd notices the change + 5 seconds to
	// accumulate data in the ring buffer.
	time.Sleep((2 + 5) * time.Second)

	// Send an OOM-kill notification.
	FakeOomKillSignal(s)

	// Wait at least 5 seconds for a clip file to be dumped.
	time.Sleep((5 + 1) * time.Second)

	// Verify that the clip file has been dumped.  Don't check for Glob
	// errors again.
	files, _ = filepath.Glob("/var/log/memd/memd.clip*.log")
	if len(files) == 0 {
		s.Error("memd did not create a clip file")
	}
	// Success!  We don't check for the file content here, since that's
	// done in the unit tests.
}

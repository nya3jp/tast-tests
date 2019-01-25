// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MidiClient,
		Desc:         "Checks MIDI Apps can send messages to devices",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"ArcMidiClientTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

// Helper function to get the port number of the "Midi Through" device.
// It returns the string containing the port number on success. On failure,
// an empty string is returned.
func getMidiPort(s *testing.State) string {
	out, err := exec.Command("/usr/bin/arecordmidi", "-l").Output()
	if err != nil {
		s.Fatal("Couldn't start arecordmidi: ", err)
	}

	// The output of arecordmidi is assumed to be of the following format:
	//
	// Port    Client name                      Port name
	// 14:0    Midi Through                     Midi Through Port-0
	//
	// So, we parse the output string and search for the port associated
	// with "Midi Through" assuming the above.
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Split(line, "   ")
		client := strings.TrimSpace(fields[1])
		if strings.Compare(client, "Midi Through") == 0 {
			// Return the port.
			return strings.TrimSpace(fields[0])
		}
	}
	return ""
}

func MidiClient(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	port := getMidiPort(s)
	if len(port) <= 0 {
		s.Fatal("Couldn't find MIDI port.")
	}

	// Create a temporary dir to store the MIDI output.
	td, err := ioutil.TempDir("", "tast.arc.midi")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	if err := os.Chmod(td, 0755); err != nil {
		s.Fatalf("Failed to set permissions on %v: %v", td, err)
	}

	// Start listening on the port for MIDI messages.
	midiFile := filepath.Join(td, "arc_midi")
	cmd := exec.Command("/usr/bin/arecordmidi", "-p", port, midiFile)
	if err := cmd.Start(); err != nil {
		s.Fatal("Couldn't start areceordmidi capture: ", err)
	}
	s.Log("Starting arecordmidi for port: ", port)

	const (
		apk = "ArcMidiClientTest.apk"
		pkg = "org.chromium.arc.testapp.arcmidiclienttest"
		cls = "org.chromium.arc.testapp.arcmidiclienttest.MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	// Wait for 2 seconds for the app to start and write the MIDI message before
	// killing the BG task.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(2 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			s.Fatal("Couldn't kill arecordmidi task: ", err)
		}
	case err := <-done:
		if err != nil {
			s.Fatal("arecordmidi task finished with error: ", err)
		}
	}
	s.Log("Stopped arecordmidi.")

	// TODO: Inspect contents of |midiFile| here.
}

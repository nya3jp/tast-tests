// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MIDIClient,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks MIDI Apps can send messages to devices",
		Contacts:     []string{"pmalani@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func MIDIClient(ctx context.Context, s *testing.State) {
	port, err := getMIDIPort(ctx)
	if err != nil {
		s.Fatal("Couldn't find MIDI port: ", err)
	}

	// Start listening on the port for MIDI messages.
	midiFile := filepath.Join(s.OutDir(), "arc.midi")
	cmd := testexec.CommandContext(ctx, "/usr/bin/arecordmidi", "-n", "1", "-p", port, midiFile)
	if err := cmd.Start(); err != nil {
		s.Fatal("Couldn't start areceordmidi capture: ", err)
	}
	defer cmd.Wait()
	s.Log("Starting arecordmidi for port ", port)

	a := s.FixtValue().(*arc.PreData).ARC

	const (
		apk = "ArcMidiClientTest.apk"
		pkg = "org.chromium.arc.testapp.arcmidiclient"
		cls = "org.chromium.arc.testapp.arcmidiclient.MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	// Wait for the app to start and arecordmidi to receive the MIDI message
	// before signalling an error.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		fi, err := os.Stat(midiFile)
		if err != nil {
			return err
		}
		if fi.Size() == 0 {
			return errors.Errorf("file %s is empty", midiFile)
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond})
	if err != nil {
		s.Fatal("arecordmidi failed to receive MIDI message: ", err)
	}

	// Release all resources associated with the command, but don't fail if there is
	// error.
	if err := cmd.Wait(); err != nil {
		s.Log("Wait() for arecordmidi failed: ", err)
	}

	// Expected values from the recorded messages.
	const minLen = 43                      // minimum expected bytes to be read.
	var expectedMsg = []byte{146, 60, 127} // NoteOn MIDI event.

	midiBytes, err := ioutil.ReadFile(midiFile)
	if err != nil {
		s.Fatalf("Failed to read file %s generated by arecordmidi: %s", midiFile, err)
	}

	if len(midiBytes) < minLen {
		s.Fatalf("Recorded MIDI message smaller size (%d) than expected (%d)", len(midiBytes), minLen)
	}

	// Strip the header and other events, to extract just the track data.
	rawMessage := midiBytes[37:]
	// Advance past the end of the variable-length delta-time before the message.
	start := 0
	for ; start < len(rawMessage); start++ {
		if rawMessage[start]&0x80 == 0 {
			start++
			break
		}
	}

	if start+3 > len(rawMessage) {
		s.Fatalf("Received MIDI file is malformed, starting index %d larger than expected", start)
	}

	midiMsg := rawMessage[start : start+3]
	if !bytes.Equal(midiMsg, expectedMsg) {
		s.Fatalf("Got MIDI message %v; want %v", midiMsg, expectedMsg)
	}
}

// getMIDIPort returns the port number of the "Midi Through" device as a string.
// On failure, an empty string is returned.
//
// Ports returned are of the following format:
// <client_number>:<port_number>
// where both the above numbers are the assigned to the MIDI port according
// to the ALSA sequencer interface. More information can be obtained from:
// https://linux.die.net/man/1/arecordmidi
func getMIDIPort(ctx context.Context) (string, error) {
	const emptyPort = ""
	out, err := testexec.CommandContext(ctx, "/usr/bin/arecordmidi", "-l").Output()
	if err != nil {
		return emptyPort, errors.Wrap(err, "couldn't start arecordmidi")
	}

	const MIDIClientName = "Midi Through"
	// The output of arecordmidi is assumed to be of the following format:
	//
	// Port    Client name                      Port name
	// 14:0    Midi Through                     Midi Through Port-0
	//
	// So, we parse the output string and search for the port associated
	// with "Midi Through" assuming the above.
	re := regexp.MustCompile(`(\d+:\d+)\s{2,}(.+)\s{2,}`)
	for _, line := range strings.Split(string(out), "\n") {
		fields := re.FindStringSubmatch(line)
		if fields == nil {
			continue
		}
		client := strings.TrimSpace(fields[2])
		if client == MIDIClientName {
			// Return the port.
			return strings.TrimSpace(fields[1]), nil
		}
	}
	return emptyPort, errors.Errorf("%q client not found", MIDIClientName)
}

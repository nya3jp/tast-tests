// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ds3 contains test to check the correct functioning of the
// dual shock 3 controller.
package ds3

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/uhid"
	"chromiumos/tast/testing"
)

const ds3HidRecording = "ds3.hid"

func init() {
	testing.AddTest(&testing.Test{
		Func:         DS3,
		Desc:         "Checks that the DS3 mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "cros-input@google.com", "ricardoq@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ds3.hid", "ds3Replay.html"},
		Timeout:      5 * time.Minute,
	})
}

// deviceInfoSet is used to determine when the get report requests by
// the kernel are done.
var deviceInfoSet = false

func DS3(ctx context.Context, s *testing.State) {
	d, err := createDS3(ctx, s)
	if err != nil {
		s.Fatal("Failed to create ds3: ", err)
	}
	s.Log("created controller")

	for !deviceInfoSet {
		if err := d.Dispatch(ctx); err != nil {
			// If no event was caught the kernel does not require a
			// GetReportReply, we can proceed with the test.
			if err.Error() == uhid.NoEventError {
				s.Log("No get report requests made by kernel")
				break
			}
			s.Fatal("Failed during kernel communication: ", err)
		}
	}
	s.Log("kernel information requests done")

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	conn, err := waitForScriptReady(ctx, server, s)
	if err != nil {
		s.Fatal("Failed creating connection: ", err)
	}
	s.Log("script ready")
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	expectedButtons := []string{
		"triangle",
		"circle",
		"x",
		"square",
		"top dpad",
		"right dpad",
		"bottom dpad",
		"left dpad",
		"R1",
		"L1",
		"R3",
		"L3",
		"start",
		"select",
	}

	s.Log("starting replay and js test")
	replayDS3(ctx, d, s.DataPath(ds3HidRecording))
	js := make(chan error)
	go runJavascriptTest(ctx, conn, expectedButtons, js, s)
	replayDS3(ctx, d, s.DataPath(ds3HidRecording))
	d.Destroy()

	if err, ok := <-js; ok {
		s.Fatal("Javascript test failed: ", err)
	}
}

// createDS3 creates the dualshock3 device with the proper uniq and
// sets the correct event handlers for kernel events.
func createDS3(ctx context.Context, s *testing.State) (*uhid.Device, error) {
	file, err := os.Open(s.DataPath(ds3HidRecording))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var d *uhid.Device
	if d, err = uhid.Recorded(ctx, file); err != nil {
		return nil, err
	}
	copy(d.Data.Uniq[:], uniq())
	if err = d.NewKernelDevice(ctx); err != nil {
		return nil, err
	}
	d.EventHandlers[uhid.GetReport] = getReport
	return d, nil
}

// replayDS3Async replays the events from the given file in the given
// device.
func replayDS3Async(ctx context.Context, d *uhid.Device, path string, c chan error, s *testing.State) {
	file, err := os.Open(path)
	if err != nil {
		c <- err
	}
	defer file.Close()
	s.Log("starting replay")
	if err := d.Replay(ctx, file); err != nil {
		c <- err
	}
	s.Log("finished replay")
	close(c)
}

func replayDS3(ctx context.Context, d *uhid.Device, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := d.Replay(ctx, file); err != nil {
		return err
	}
	return nil
}

// uniq returns a randomly generated uniq string for a dualshock3. The
// uniq must be composed of a string of 6 colon-separated unsigned 8
// bit hexadecimal integers for the dualshock 3 to properly function.
func uniq() string {
	var rands [6]string
	for i := 0; i < 6; i++ {
		rands[i] = fmt.Sprintf("%02x", rand.Intn(256))
	}
	return strings.Join(rands[:], ":")
}

// getReport handles getReport requests by the kernel.
func getReport(ctx context.Context, d *uhid.Device, buf []byte) error {
	reader := bytes.NewReader(buf)
	event := uhid.GetReportRequest{}
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return err
	}
	data, err := processRNum(d, event.RNum)
	if err != nil {
		return err
	}
	reply := uhid.GetReportReplyRequest{
		RequestType: uhid.GetReportReply,
		ID:          event.ID,
		Err:         0,
		DataSize:    uint16(len(data)),
	}
	copy(reply.Data[:], data[:])
	return d.WriteEvent(reply)
}

// processRNum returns the data that will be written in the get report
// reply depending on the rnum that was sent.
func processRNum(d *uhid.Device, rnum uint8) ([]byte, error) {
	if rnum == uint8(0xf2) {
		return f2RNumData(d.Data.Uniq[:])
	} else if rnum == 0xf5 {

		// the creation of a dualshock 3 will entail 2 rnum=0xf2 get
		// report requests and one rnum=0xf5 after that. Therefore, the
		// dispatching ends once we receive the 0xf5 request.
		deviceInfoSet = true
		return []byte{0x01, 0x00, 0x18, 0x5e, 0x0f, 0x71, 0xa4, 0xbb}, nil
	}
	return []byte{}, nil
}

// f2RNumData returns the data that will be written in the get report
// reply for rnum=0xf2.
func f2RNumData(uniq []byte) ([]byte, error) {
	// undocumented report in the HID report descriptor: the MAC address
	// of the device is stored in the bytes 4 to 9, the rest has been
	// dumped on a Sixaxis controller.
	data := []byte{0xf2, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x40, 0x80, 0x18, 0x01, 0x8a}
	for i, v := range strings.Split(string(uniq), ":") {
		n, err := strconv.ParseUint(v[:2], 16, 8)
		if err != nil {
			return nil, err
		}
		data[i+4] = uint8(n)
	}
	return data, nil
}

func waitForScriptReady(ctx context.Context, server *httptest.Server, s *testing.State) (*chrome.Conn, error) {
	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := cr.NewConn(ctx, path.Join(server.URL, "ds3Replay.html"))
	if err != nil {
		return nil, err
	}

	if err := conn.WaitForExpr(ctx, "scriptReady"); err != nil {
		return nil, err
	}

	return conn, nil
}

func runJavascriptTest(ctx context.Context, conn *chrome.Conn, expected []string, c chan error, s *testing.State) {
	if err := conn.WaitForExpr(ctx, "gamepadDisconnected"); err != nil {
		c <- err
		return
	}

	var actual []string
	if err := conn.Eval(ctx, "getResults()", &actual); err != nil {
		c <- err
		return
	}

	if err := compareButtonSequence(expected, actual); err != nil {
		c <- err
		return
	}

	close(c)
}

func compareButtonSequence(expected, actual []string) error {
	if len(expected) != len(actual) {
		return errors.Errorf("expected button array length and actual button array lengths differ: got %d, wanted %d", len(actual), len(expected))
	}
	for i, v := range expected {
		if v != actual[i] {
			return errors.Errorf("Button discrepancy at index %d: got %s, wanted %s", i, actual[i], v)
		}
	}
	return nil
}

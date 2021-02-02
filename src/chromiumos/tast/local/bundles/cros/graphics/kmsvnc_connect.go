// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"encoding/binary"
	"net"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KmsvncConnect,
		Desc:         "Connects to kmsvnc server and verifies handshake messages",
		Contacts:     []string{"shaochuan@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
	})
}

// KmsvncConnect launches the kmsvnc server, connects to it, and verifies initial handshake messages as defined by the RFB protocol (RFC6143).
// https://tools.ietf.org/html/rfc6143#section-7
func KmsvncConnect(ctx context.Context, s *testing.State) {
	// Run kmsvnc in the background, and make sure it's terminated when the test finishes,
	cmd := testexec.CommandContext(ctx, "kmsvnc")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start kmsvnc: ", err)
	}
	defer func() {
		// Send SIGTERM so kmsvnc may exit gracefully.
		// In case that fails the watchdoc routine created by cmd.Start() will kill it for us.
		if err := cmd.Signal(syscall.SIGTERM); err != nil {
			s.Fatal("Failed to send SIGTERM to kmsvnc: ", err)
		}
		if err := cmd.Wait(); err != nil {
			s.Error("kmsvnc exited with error: ", err)
		}
		cmd.DumpLog(ctx)
	}()
	// Sleep to ensure the subprocess is started.
	testing.Sleep(ctx, time.Second)

	w, h := findDisplayWidthHeight(ctx, s)
	s.Logf("Found primary display size %dx%d", w, h)

	conn, err := net.Dial("tcp", "localhost:5900")
	if err != nil {
		s.Fatal("Failed to connect to localhost:5900: ", err)
	}
	defer conn.Close()

	expectProtocolVersionHandshake(ctx, s, conn)
	expectSecurityHandshake(ctx, s, conn)
	expectInitHandshake(ctx, s, conn, w, h)
}

// findDisplayWidthHeight returns the width/height of the primary display, which should match the VNC framebuffer size.
func findDisplayWidthHeight(ctx context.Context, s *testing.State) (int, int) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
	}

	dm, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get selected display mode: ", err)
	}

	// TODO: handle rotation on tablet devices?
	return dm.WidthInNativePixels, dm.HeightInNativePixels
}

// expectProtocolVersionHandshake verifies ProtocolVersion handshake messages.
func expectProtocolVersionHandshake(ctx context.Context, s *testing.State, conn net.Conn) {
	got := make([]byte, 12)
	if _, err := conn.Read(got); err != nil {
		s.Fatal("Error receiving ProtocolVersion message: ", err)
	}
	want := []byte("RFB 003.008\n")
	if !cmp.Equal(got, want) {
		s.Fatalf("Unexpected ProtocolVersion message, got %v, want %v", got, want)
	}

	if _, err := conn.Write([]byte("RFB 003.008\n")); err != nil {
		s.Fatal("Error sending ProtocolVersion response: ", err)
	}
}

// expectSecurityHandshake verifies security handshake messages.
func expectSecurityHandshake(ctx context.Context, s *testing.State, conn net.Conn) {
	got := make([]byte, 2)
	if _, err := conn.Read(got); err != nil {
		s.Fatal("Error receiving security types: ", err)
	}
	// number-of-security-types = 1, security-types = [None]
	want := []byte{0x1, 0x1}
	if !cmp.Equal(got, want) {
		s.Fatalf("Unexpected security types, got %v, want %v", got, want)
	}

	// Response: security-type = None
	if _, err := conn.Write([]byte{0x1}); err != nil {
		s.Fatal("Error sending security type response: ", err)
	}

	got = make([]byte, 4)
	if _, err := conn.Read(got); err != nil {
		s.Fatal("Error receiving SecurityResult message: ", err)
	}
	// SecurityResult = OK
	want = []byte{0, 0, 0, 0}
	if !cmp.Equal(got, want) {
		s.Fatalf("Unexpected SecurityResult message, got %v, want %v", got, want)
	}
}

// expectInitHandshake sends a ClientInit message, and verifies the ServerInit response.
func expectInitHandshake(ctx context.Context, s *testing.State, conn net.Conn, wantW, wantH int) {
	if _, err := conn.Write([]byte{0x1}); err != nil {
		s.Fatal("Error sending ClientInit message: ", err)
	}

	// ServerInit is of variable length, we only need the first 17 bytes.
	buf := make([]byte, 17)
	if _, err := conn.Read(buf); err != nil {
		s.Fatal("Error receiving ServerInit message: ", err)
	}

	// Verify framebuffer size.
	gotW := int(binary.BigEndian.Uint16(buf[0:2]))
	gotH := int(binary.BigEndian.Uint16(buf[2:4]))
	if gotW != wantW || gotH != wantH {
		s.Errorf("Unexpected framebuffer size, got %dx%d, want %dx%d", gotW, gotH, wantW, wantH)
	}

	// Verify pixel format.
	got := buf[4:17]
	want := []byte{
		0x20,       // bits-per-pixel
		0x20,       // depth
		0x00,       // big-endian-flag
		0xff,       // true-color-flag
		0x00, 0xff, // red-max
		0x00, 0xff, // green-max
		0x00, 0xff, // blue-max
		0x10, // red-shift
		0x08, // green-shift
		0x00, // blue-shift
	}
	if !cmp.Equal(got, want) {
		s.Errorf("Unexpected pixel format, got %v, want %v", got, want)
	}
}

// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kmsvnc is a library for wrapping the kmsvnc binary from tast.
package kmsvnc

import (
	"bufio"
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	kmsvncHostPort     = "localhost:5900"
	kmsvncReadyMessage = "Listening for VNC connections"
	kmsvncReadyTimeout = 3 * time.Second
	kmsvncStopTimeout  = 3 * time.Second

	rfbProtocolVersion = "RFB 003.008\n"
)

// Kmsvnc wraps a kmsvnc process used in tests.
type Kmsvnc struct {
	cmd  *testexec.Cmd
	conn net.Conn
}

// NewKmsvnc launches kmsvnc as a subprocess and returns a handle.
// Blocks until kmsvnc is ready to accept connections, so it's safe to connect to kmsvnc once this function returns.
func NewKmsvnc(ctx context.Context, verbose bool) (*Kmsvnc, error) {
	cmd := testexec.CommandContext(ctx, "kmsvnc")

	// Create a pipe for stderr which we'll be listening later.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Launch a separate goroutine to listen stderr and print as logs.
	// Once kmsvnc is ready to accept connections, `true` is sent to the |ready| channel.
	// OTOH, if the scanner receives an EOF before ready i.e. the process exits, `false` is sent to the channel.
	// TODO(b/177965296): Save logs to separate file.
	ready := make(chan bool)
	go func(ready chan<- bool) {
		logger := func(s string) {
			testing.ContextLog(ctx, "kmsvnc: ", s)
		}
		if !verbose {
			outDir, ok := testing.ContextOutDir(ctx)
			if !ok {
				testing.ContextLog(ctx, "Unable to determine the output directory")
			} else {
				outFile := filepath.Join(outDir, "kmsvnc.log")

				file, err := os.Create(outFile)
				if err != nil {
					testing.ContextLog(ctx, "Failed to open kmsvnc log file for appending: ", err)
				}
				defer file.Close()

				logger = func(s string) {
					_, err := file.WriteString(s + "\n")
					if err != nil {
						testing.ContextLog(ctx, "Failed to write string to kmsvnc log file: ", err)
					}
				}
			}
		}

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t := scanner.Text()
			logger(t)
			if ready != nil && strings.Contains(t, kmsvncReadyMessage) {
				ready <- true
				close(ready)
				ready = nil
			}
		}
		if err := scanner.Err(); err != nil {
			testing.ContextLog(ctx, "Error reading kmsvnc stderr: ", scanner.Err())
		}
		if ready != nil {
			ready <- false
			close(ready)
		}
	}(ready)

	// Block until kmsvnc is ready, or fail if context timed out / startup took too long.
	// Make a child context so existing timeouts are taken into account.
	tctx, cancel := context.WithTimeout(ctx, kmsvncReadyTimeout)
	defer cancel()

	// Kill the process and cleanup in another goroutine in case of failures.
	cleanup := func() {
		if err := cmd.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill kmsvnc process: ", err)
		}
		go cmd.Wait()
	}

	select {
	case <-tctx.Done():
		cleanup()
		return nil, tctx.Err()
	case ok := <-ready:
		if !ok {
			cleanup()
			return nil, errors.New("kmsvnc process exited unexpectedly, check logs for details")
		} else if !verbose {
			testing.ContextLog(ctx, "kmsvnc is ready. Logging to kmsvnc.log")
		}
		return &Kmsvnc{cmd: cmd}, nil
	}
}

// Stop closes existing connections and terminates the kmsvnc process gracefully.
func (k *Kmsvnc) Stop(ctx context.Context) error {
	if k.conn != nil {
		k.conn.Close()
		k.conn = nil
	}
	// In case this fails, the watchdog routine created by cmd.Start() will kill it when the context expires.
	if err := k.cmd.Signal(unix.SIGTERM); err != nil {
		return err
	}

	// Attempt to cleanup the process, or timeout if that takes too long.
	done := make(chan struct{})
	go func() {
		k.cmd.Wait()
		close(done)
	}()
	tctx, cancel := context.WithTimeout(ctx, kmsvncStopTimeout)
	defer cancel()
	select {
	case <-tctx.Done():
		return tctx.Err()
	case <-done:
		return nil
	}
}

// RFBServerInit represents a ServerInit message as specified in RFB protocol. Only fields needed for testing are included.
// https://tools.ietf.org/html/rfc6143#section-7.3.2
type RFBServerInit struct {
	FramebufferWidth  uint16
	FramebufferHeight uint16
	PixelFormat       []byte
}

// Connect connects to the kmsvnc server, completes the initial handshake as defined in RFC6143, and returns server parameters.
// https://tools.ietf.org/html/rfc6143#section-7
func (k *Kmsvnc) Connect(ctx context.Context) (*RFBServerInit, error) {
	if k.conn != nil {
		return nil, errors.New("already connected")
	}

	conn, err := net.Dial("tcp", kmsvncHostPort)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", kmsvncHostPort)
	}
	k.conn = conn

	if err := k.expectProtocolVersionHandshake(ctx); err != nil {
		return nil, errors.Wrap(err, "failed ProtocolVersion handshake")
	}
	if err := k.expectSecurityHandshake(ctx); err != nil {
		return nil, errors.Wrap(err, "failed Security handshake")
	}
	serverInit, err := k.initHandshake(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed Initialization handshake")
	}

	return serverInit, nil
}

// expectProtocolVersionHandshake verifies ProtocolVersion handshake messages.
func (k *Kmsvnc) expectProtocolVersionHandshake(ctx context.Context) error {
	got := make([]byte, 12)
	if _, err := k.conn.Read(got); err != nil {
		return errors.Wrap(err, "error receiving ProtocolVersion message")
	}
	want := []byte(rfbProtocolVersion)
	if !cmp.Equal(got, want) {
		return errors.Errorf("unexpected ProtocolVersion message, got %v, want %v", got, want)
	}

	if _, err := k.conn.Write([]byte(rfbProtocolVersion)); err != nil {
		return errors.Wrap(err, "error sending ProtocolVersion response")
	}

	return nil
}

// expectSecurityHandshake verifies security handshake messages.
func (k *Kmsvnc) expectSecurityHandshake(ctx context.Context) error {
	got := make([]byte, 2)
	if _, err := k.conn.Read(got); err != nil {
		return errors.Wrap(err, "error receiving security types")
	}
	// number-of-security-types = 1, security-types = [None]
	want := []byte{0x1, 0x1}
	if !cmp.Equal(got, want) {
		return errors.Errorf("unexpected security types, got %v, want %v", got, want)
	}

	// Response: security-type = None
	if _, err := k.conn.Write([]byte{0x1}); err != nil {
		return errors.Wrap(err, "error sending security type response")
	}

	got = make([]byte, 4)
	if _, err := k.conn.Read(got); err != nil {
		return errors.Wrap(err, "error receiving SecurityResult message")
	}
	// SecurityResult = OK
	want = []byte{0, 0, 0, 0}
	if !cmp.Equal(got, want) {
		return errors.Errorf("unexpected SecurityResult message, got %v, want %v", got, want)
	}

	return nil
}

// initHandshake sends a ClientInit message, and parses the ServerInit response.
func (k *Kmsvnc) initHandshake(ctx context.Context) (*RFBServerInit, error) {
	// ClientInit: shared-flag = 1
	if _, err := k.conn.Write([]byte{0x1}); err != nil {
		return nil, errors.Wrap(err, "error sending ClientInit message")
	}

	// ServerInit is of variable length, we only need the first 17 bytes.
	buf := make([]byte, 17)
	if _, err := k.conn.Read(buf); err != nil {
		return nil, errors.Wrap(err, "error receiving ServerInit message")
	}

	return &RFBServerInit{
		FramebufferWidth:  binary.BigEndian.Uint16(buf[0:2]),
		FramebufferHeight: binary.BigEndian.Uint16(buf[2:4]),
		PixelFormat:       buf[4:17],
	}, nil
}

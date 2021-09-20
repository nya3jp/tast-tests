// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
	"io"
	"os/exec"
	"regexp"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// command unifies exec and dut commands for use in CreatePtyPair.
type command interface {
	Start() error
	StderrPipe() (io.ReadCloser, error)
	Wait() error
	Abort()
}
type execCommand struct {
	*exec.Cmd
}

func (c *execCommand) Abort() {
	c.Cmd.Process.Kill()
}

type sshCommand struct {
	*ssh.Cmd
}

func (c *sshCommand) Wait() error {
	return c.Cmd.Wait()
}

// CreateDUTPTYPair creates a pair of connected ptys on the DUT and returns
// their names, along with cancel and done to manage their lifetimes.
func CreateDUTPTYPair(ctx context.Context, dut *dut.DUT) (s1, s2 string, cancel func(), done <-chan error, err error) {
	cmd := &sshCommand{dut.Conn().CommandContext(ctx, "socat", "-d", "-d", "pty,raw,echo=0", "pty,raw,echo=0")}
	return createPTYPair(ctx, cmd)
}

// CreateHostPTYPair creates a pair of connected ptys on the host and returns
// their names, along with cancel and done to manage their lifetimes.
func CreateHostPTYPair(ctx context.Context) (s1, s2 string, cancel func(), done <-chan error, err error) {
	cmd := &execCommand{exec.CommandContext(ctx, "socat", "-d", "-d", "pty,raw,echo=0", "pty,raw,echo=0")}
	return createPTYPair(ctx, cmd)
}

// createPTYPair creates a pair of connected ptys and returns their names,
// along with cancel and done to manage their lifetimes.
func createPTYPair(ctx context.Context, cmd command) (s1, s2 string, cancel func(), done <-chan error, err error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", nil, nil, errors.New("could not obtain stderrpipe")
	}
	err = cmd.Start()
	if err != nil {
		return "", "", nil, nil, errors.New("could not start process")
	}
	ptyDone := make(chan error, 1)
	ptyRE := regexp.MustCompile(`(?s)PTY is (\S*).*PTY is (\S*)`)

	ptyFound := make(chan struct{}, 1)
	// Find ptys from stderr then wait for cmd to finish.
	go func() {
		buf := make([]byte, 200)
		total := 0
		for {
			n, err := stderr.Read(buf[total:])
			if err != nil {
				cmd.Abort()
				break
			}
			total += n
			m := ptyRE.FindSubmatch(buf[:total])
			if len(m) > 0 {
				s1 = string(m[1])
				s2 = string(m[2])
				ptyFound <- struct{}{}
				break
			}
			if total == 200 {
				cmd.Abort()
				break
			}
		}
		// if ptyFound, cmd.Wait will block until caller calls cancel.
		ptyDone <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		cmd.Abort()
		return "", "", nil, nil, ctx.Err()
	case <-ptyFound:
		return s1, s2, cmd.Abort, ptyDone, nil
	case <-ptyDone:
		return "", "", nil, nil, errors.New("could not obtain pty pair")
	}
}

// DoTestRead tests Port.Read.
// This is only used by integration tests due to the longer run time caused by
// a timeout from an intentional read operation when no data is available.
func DoTestRead(ctx context.Context, logf func(...interface{}), p1, p2 Port) error {
	logf("Read should error when no data")
	buf := make([]byte, 4)
	ctx1, ctx1Cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer ctx1Cancel()
	if n, err := p2.Read(ctx1, buf); err == nil {
		return errors.New("read succeeded unexpectedly")
	} else if n != 0 {
		return errors.Errorf("read returned %d, want 0", n)
	}

	// Write data to port to unblock the read go routine.
	if n, err := p1.Write(ctx, []byte("abcdefg")); err != nil {
		return errors.Wrap(err, "write 7 bytes")
	} else if n != 7 {
		return errors.Errorf("write returned %d, want 7", n)
	}
	buf = make([]byte, 7)
	// Verify that the last bytes of written data have been received.
	if n, err := p2.Read(ctx, buf); err != nil {
		return errors.Wrap(err, "read 7 bytes")
	} else if n < 3 {
		return errors.Errorf("read returned %d, want >= 3", n)
	} else if string(buf[n-3:n]) != "efg" {
		return errors.Errorf("last 3 bytes, got %q, want \"efg\"", string(buf[n-3:n]))
	}
	return nil
}

// DoTestWrite tests Port.Write.
// This is used by both unit and integration tests and so should finish promptly.
func DoTestWrite(ctx context.Context, logf func(...interface{}), p1, p2 Port) error {
	logf("Write should work")
	if n, err := p1.Write(ctx, []byte("abc")); err != nil {
		return errors.Wrap(err, "write")
	} else if n != 3 {
		return errors.Errorf("bytes written, got %d, want 3", n)
	}

	logf("Should be able to read written data")
	buf := make([]byte, 4)
	if n, err := p2.Read(ctx, buf); err != nil {
		return errors.Wrap(err, "read")
	} else if n != 3 {
		return errors.Errorf("read returned %d, want 3", n)
	} else if string(buf[:3]) != "abc" {
		return errors.Errorf("read buffer %q, want \"abc\"", string(buf[:3]))
	}

	return nil
}

// DoTestFlush tests Port.Flush.
// This is used by both unit and integration tests and so should finish promptly.
func DoTestFlush(ctx context.Context, logf func(...interface{}), p1, p2 Port) error {
	logf("Flush should work")
	if n, err := p1.Write(ctx, []byte("abc")); err != nil {
		return errors.Wrap(err, "write \"abc\"")
	} else if n != 3 {
		return errors.Errorf("bytes written, got %d, want %d", n, 3)
	}
	buf := make([]byte, 1)
	if _, err := p2.Read(ctx, buf); err != nil {
		return errors.Wrap(err, "reading first byte after write")
	}
	if err := p2.Flush(ctx); err != nil {
		return errors.Wrap(err, "flush")
	}
	if n, err := p1.Write(ctx, []byte("def")); err != nil {
		return errors.Wrap(err, "write \"def\"")
	} else if n != 3 {
		return errors.Errorf("write returned %d, want 3", n)
	}
	buf = make([]byte, 4)
	if n, err := p2.Read(ctx, buf); err != nil {
		return errors.Wrap(err, "read after flush")
	} else if n != 3 {
		return errors.Errorf("read returned %d, want 3", n)
	} else if string(buf[:3]) != "def" {
		return errors.Errorf("read buffer, got %q, want \"def\"", string(buf[:3]))
	}

	logf("Close after flush should work")
	if err := p2.Close(ctx); err != nil {
		return errors.Wrap(err, "close")
	}
	return nil
}

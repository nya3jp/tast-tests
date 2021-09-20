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
	"chromiumos/tast/testing"
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
func DoTestRead(ctx context.Context, logf func(...interface{}), p1, p2 Port) error {
	logf("Read should error when no data")
	buf := make([]byte, 4)
	ctx1, ctx1Cancel := context.WithTimeout(ctx, time.Duration(200*time.Millisecond))
	defer ctx1Cancel()
	n, err := p2.Read(ctx1, buf)
	if err == nil {
		return errors.New("read succeeded unexpectedly")
	}
	if n != 0 {
		return errors.Errorf("read returned %d, want 0", n)
	}

	// Write data to port to unblock the read go routine.
	n, err = p1.Write(ctx, []byte("abcdefg"))
	if err != nil {
		return errors.Wrap(err, "write 7 bytes")
	}
	if n != 7 {
		return errors.Errorf("write returned %d, want 7", n)
	}
	buf = make([]byte, 7)
	total := 0
	// Verify that the last bytes of written data have been received.
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		n, err := p2.Read(ctx, buf[total:])
		if err != nil {
			return err
		}
		total += n
		if total >= 3 && string(buf[total-3:total]) == "efg" {
			return nil
		}
		return errors.New("last written bytes not found yet")
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 10 * time.Millisecond}); err != nil {
		return err
	}
	return nil
}

// DoTestWrite tests Port.Write.
func DoTestWrite(ctx context.Context, logf func(...interface{}), p1, p2 Port) error {
	logf("Write should work")
	n, err := p1.Write(ctx, []byte("abc"))
	if err != nil {
		return errors.Wrap(err, "write errored")
	}
	if n != 3 {
		return errors.Errorf("bytes written, want %d, got %d", 3, n)
	}

	logf("Should be able to read written data")
	buf := make([]byte, 4)
	n, err = p2.Read(ctx, buf)
	if err != nil {
		return errors.Wrap(err, "read errored")
	}
	if n != 3 || string(buf[:n]) != "abc" {
		return errors.Errorf("unexpected read result, n: %d, buf: %q", n, buf)
	}

	return nil
}

// DoTestFlush tests Port.Flush.
func DoTestFlush(ctx context.Context, logf func(...interface{}), p1, p2 Port) error {
	logf("Flush should work")
	n, err := p1.Write(ctx, []byte("abc"))
	if err != nil {
		return errors.Wrap(err, "write")
	}
	if n != 3 {
		return errors.Errorf("bytes written, want %d, got %d", 3, n)
	}
	buf := make([]byte, 1)
	_, err = p2.Read(ctx, buf)
	if err != nil {
		return errors.Wrap(err, "reading first byte after write")
	}
	err = p2.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "flush")
	}
	n, err = p1.Write(ctx, []byte("def"))
	if err != nil || n != 3 {
		return errors.Wrapf(err, "unexpected write result, n: %d", n)
	}
	buf = make([]byte, 4)
	n, err = p2.Read(ctx, buf)
	if err != nil || n != 3 || string(buf[:n]) != "def" {
		return errors.Errorf("unexpected read result, n: %d, err: %v, buf: %q", n, err, string(buf[:n]))
	}

	logf("Close after flush should work")
	err = p2.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close errored")
	}
	return nil
}

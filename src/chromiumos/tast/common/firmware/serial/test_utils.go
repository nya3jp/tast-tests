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

// iCmd unifies exec and dut commands for use in CreatePtyPair.
type iCmd interface {
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

type ctxCommand struct {
	*ssh.Cmd
}

func (c *ctxCommand) Wait() error {
	return c.Cmd.Wait()
}

// CreatePtyPair creates a pair of connected ptys and returns their names,
// along with cancel and done to manage their lifetimes.
func CreatePtyPair(ctx context.Context, dut *dut.DUT) (s1, s2 string, cancelf func(), done <-chan error, err error) {
	var cmd iCmd
	if dut == nil {
		cmd = &execCommand{exec.Command("socat", "-d", "-d", "pty,raw,echo=0", "pty,raw,echo=0")}
	} else {
		cmd = &ctxCommand{dut.Conn().CommandContext(ctx, "socat", "-d", "-d", "pty,raw,echo=0", "pty,raw,echo=0")}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", nil, nil, errors.New("could not obtain stderrpipe")
	}
	err = cmd.Start()
	if err != nil {
		return "", "", nil, nil, errors.New("could not start process")
	}
	cancel := make(chan int)

	// Kill process upon cancel.
	go func() {
		<-cancel
		cmd.Abort()
	}()

	// Send cancel signal.
	cancelf = func() {
		cancel <- 1
	}
	doneC := make(chan error)
	ptyRe := regexp.MustCompile(`(?s)PTY is (\S*).*PTY is (\S*)`)

	t1 := make(chan string)
	t2 := make(chan string)
	// Find ptys from stderr then wait for cmd to finish.
	go func() {
		buf := make([]byte, 200)
		total := 0
		for {
			n, err := stderr.Read(buf[total:])
			total += n
			m := ptyRe.FindSubmatch(buf[:total])
			if len(m) > 0 {
				t1 <- string(m[1])
				t2 <- string(m[2])
				break
			}
			if err != nil || ctx.Err() != nil || total == 200 {
				cancelf()
				t1 <- ""
				t2 <- ""
				break
			}
		}
		doneC <- cmd.Wait()
	}()

	// Wait for the ptys.
	s1 = <-t1
	s2 = <-t2

	if s1 == "" {
		return "", "", nil, doneC, errors.New("could not obtain pty pair")
	}
	return s1, s2, cancelf, doneC, nil
}

// DoTestPort tests serial ports opened by provided openers.
func DoTestPort(ctx context.Context, logf func(...interface{}), o1, o2 PortOpener) error {
	logf("Open should work")
	p1, err := o1.OpenPort(ctx)
	if err != nil {
		return errors.Wrap(err, "open 1 errored")
	}
	defer p1.Close(ctx)

	p2, err := o2.OpenPort(ctx)
	if err != nil {
		return errors.Wrap(err, "open 2 errored")
	}
	defer p2.Close(ctx)

	logf("Write should work")
	n, err := p1.Write(ctx, []byte("abc"))
	if err != nil {
		return errors.Wrap(err, "write errored")
	}
	if n != 3 {
		return errors.Errorf("bytes written, want %d, got %d", 3, n)
	}

	logf("Read should work")
	buf := []byte("wxyz")
	n, err = p2.Read(ctx, buf)
	if err != nil {
		return errors.Wrap(err, "read errored")
	}
	logf("Read in: \"" + string(buf[:n]) + "\"")
	if n != 3 {
		return errors.Errorf("bytes read, want %d, got %d", 3, n)
	}
	if string(buf[:3]) != "abc" {
		return errors.Errorf("read buffer, want %q, got %q", "abc", buf[:3])
	}

	logf("Read should return EOF when no data")
	buf = []byte("wxyz")
	n, err = p2.Read(ctx, buf)
	if err == nil {
		return errors.New("read should have failed")
	}
	if n != 0 {
		return errors.Errorf("read should have read 0 bytes: %d", n)
	}
	if string(buf) != "wxyz" {
		return errors.Errorf("read should not have modified buffer: %q", string(buf))
	}

	logf("Flush should work")
	n, err = p1.Write(ctx, []byte("abc"))
	if err != nil {
		return errors.Wrap(err, "write errored")
	}
	if n != 3 {
		return errors.Errorf("bytes written, want %d, got %d", 3, n)
	}
	testing.Sleep(ctx, 10*time.Millisecond)
	err = p2.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "flush erroed")
	}
	n, err = p2.Read(ctx, buf)
	if err == nil || n != 0 {
		return errors.Errorf("read should have failed: %v n: %d buf: %s", err, n, string(buf[:n]))
	}

	logf("Close should work")
	err = p1.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close errored")
	}
	return nil
}

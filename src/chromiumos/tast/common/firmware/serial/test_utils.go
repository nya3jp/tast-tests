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

	logf("Read should error when no data")
	buf = []byte("wxyz")
	ctx1, ctx1Cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer ctx1Cancel()
	n, err = p2.Read(ctx1, buf)
	if err == nil {
		return errors.New("read1 should have failed")
	}
	logf("Read errored as expected: " + err.Error())

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
	ctx2, ctx2Cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer ctx2Cancel()
	n, err = p2.Read(ctx2, buf)
	if err == nil || n != 0 {
		return errors.Errorf("read2 should have failed: %v n: %d buf: %s", err, n, string(buf[:n]))
	}
	logf("Read errored as expected: " + err.Error())

	logf("Flush after possible write error should work")
	ctx3, ctx3Cancel := context.WithTimeout(ctx, 10*time.Microsecond)
	defer ctx3Cancel()
	n, err = p1.Write(ctx3, []byte("abc"))
	if err != nil {
		logf("Expected write error: " + err.Error())
	} else {
		logf("Expected write success")
	}
	testing.Sleep(ctx, 50*time.Millisecond)
	err = p2.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "flush errored")
	}
	n, err = p1.Write(ctx, []byte("def"))
	if err != nil || n != 3 {
		return errors.Wrapf(err, "unexpected write result, n: %d", n)
	}
	n, err = p2.Read(ctx, buf)
	if err != nil || n != 3 || string(buf[:n]) != "def" {
		return errors.Errorf("unexpected read result, n: %d, err: %v, buf: %q", n, err, string(buf[:n]))
	}

	logf("Flush after read error should work")
	ctx4, ctx4Cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer ctx4Cancel()
	n, err = p1.Read(ctx4, buf)
	if err == nil {
		return errors.Wrap(err, "read3 should have failed")
	}
	logf("Read errored as expected: " + err.Error())
	testing.Sleep(ctx, 50*time.Millisecond)
	err = p2.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "flush errored")
	}
	n, err = p1.Write(ctx, []byte("abcdefg"))
	if err != nil || n != 7 {
		return errors.Wrapf(err, "unexpected write result, n: %d", n)
	}
	buf1 := make([]byte, 3)
	n, err = p2.Read(ctx, buf1)
	if err != nil || n != 3 || string(buf1[:n]) != "abc" {
		return errors.Errorf("unexpected read result, n: %d, err: %v, buf: %q", n, err, string(buf1[:n]))
	}
	buf2 := make([]byte, 5)
	n, err = p2.Read(ctx, buf2)
	if err != nil || n != 4 || string(buf2[:n]) != "defg" {
		return errors.Errorf("unexpected read result, n: %d, err: %v, buf: %q", n, err, string(buf2[:n]))
	}

	logf("Close should work")
	err = p1.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close errored")
	}
	return nil
}

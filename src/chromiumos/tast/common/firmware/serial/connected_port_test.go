// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
	"os/exec"
	"regexp"
	gotesting "testing"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func pty(ctx context.Context) (s1, s2 string, cancelf func(), done <-chan error, err error) {
	cmd := exec.Command("socat", "-d", "-d", "pty,raw,echo=0", "pty,raw,echo=0")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", nil, nil, errors.New("could not obtain stderrpipe")
	}
	if err = cmd.Start(); err != nil {
		return "", "", nil, nil, errors.New("could not start process")
	}
	cancel := make(chan int)

	// Kill process upon cancel.
	go func() {
		<-cancel
		cmd.Process.Kill()
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

func TestConnectedPort(t *gotesting.T) {
	ctx := context.Background()

	t.Log("Open should work")
	ctxT, ctxCancel := context.WithTimeout(ctx, 2*time.Second)
	defer ctxCancel()
	pty1, pty2, cancel, done, err := pty(ctxT)
	if err != nil {
		t.Fatal("Error creating pty: ", err)
	}
	t.Logf("Created ptys: %s %s", pty1, pty2)

	defer func() {
		cancel()
		<-done
	}()

	o1 := NewConnectedPortOpener(pty1, 115200, 10*time.Millisecond)
	p1, err := o1.OpenPort(ctx)
	if err != nil {
		t.Fatal("Open 1 errored: ", err)
	}
	defer p1.Close(ctx)

	o2 := NewConnectedPortOpener(pty2, 115200, 10*time.Millisecond)
	p2, err := o2.OpenPort(ctx)
	if err != nil {
		t.Fatal("Open 2 errored: ", err)
	}
	defer p2.Close(ctx)

	t.Log("Write should work")
	n, err := p1.Write(ctx, []byte("abc"))
	if err != nil {
		t.Fatal("Write errored: ", err)
	}
	if n != 3 {
		t.Fatalf("Bytes written, want %d, got %d", 3, n)
	}

	t.Log("Read should work")
	buf := []byte("wxyz")
	n, err = p2.Read(ctx, buf)
	if err != nil {
		t.Fatal("Read errored: ", err)
	}
	t.Logf("Read in: %q", string(buf[:n]))
	if n != 3 {
		t.Fatalf("Bytes read, want %d, got %d", 3, n)
	}
	if string(buf[:3]) != "abc" {
		t.Fatalf("Read buffer, want %q, got %q", "abc", buf[:3])
	}

	t.Log("Read should return EOF when no data")
	buf = []byte("wxyz")
	n, err = p2.Read(ctx, buf)
	if err == nil {
		t.Fatal("Read should have failed")
	}
	if n != 0 {
		t.Fatal("Read should have read 0 bytes:", n)
	}
	if string(buf) != "wxyz" {
		t.Fatalf("Read should not have modified buffer: %q", string(buf))
	}

	t.Log("Flush should work")
	n, err = p1.Write(ctx, []byte("abc"))
	if err != nil {
		t.Fatal("Write errored: ", err)
	}
	if n != 3 {
		t.Fatalf("Bytes written, want %d, got %d", 3, n)
	}
	testing.Sleep(ctx, 10*time.Millisecond)
	err = p2.Flush(ctx)
	if err != nil {
		t.Fatal("Flush erroed:", err)
	}
	n, err = p2.Read(ctx, buf)
	if err == nil || n != 0 {
		t.Fatal("Read should have failed:", err, "n:", n, "buf:", string(buf[:n]))
	}

	t.Log("Close should work")
	err = p1.Close(ctx)
	if err != nil {
		t.Fatal("Close failed:", err)
	}
	if p1.(*ConnectedPort).port != nil {
		t.Fatal("port not cleaned up:", p1.(*ConnectedPort).port)
	}
}

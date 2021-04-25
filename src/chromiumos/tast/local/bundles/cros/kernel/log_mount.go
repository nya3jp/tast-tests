// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LogMount,
		Desc:     "Checks that the kernel logs a message when a filesystem is mounted",
		Contacts: []string{"hyungtaekim@chromium.org"},
		Attr: []string{
			"group:mainline", // the default group for functional tests
			"informational",  // non-critical meaning that it is not run in CQ or PFQ
		},
	})
}

func LogMount(ctx context.Context, s *testing.State) {
	// Use a shortened context for test operations to reserrve time for cleanup.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	// Create a temp dir in /tmp to ensure that we don't leave stale mounts in
	// Tast's temp dir if we're interrupted.
	td, err := ioutil.TempDir("/tmp", "tast.kernel.LogMount.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	src := filepath.Join(td, "fs.bin")
	s.Log("Creating filesystem at ", src)
	if err := makeFilesystem(shortCtx, src, 1024*1024); err != nil {
		s.Fatal("Failed creating filesystem: ", err)
	}

	s.Log("Starting dmesg")
	dmesgCmd, dmesgCh, err := streamDmesg(shortCtx)
	if err != nil {
		s.Fatal("Failed to start dmesg: ", err)
	}
	defer dmesgCmd.Wait()
	defer dmesgCmd.Kill()

	dst := filepath.Join(td, "mnt")
	s.Logf("Mounting %v at %v", src, dst)
	if err := os.Mkdir(dst, 0755); err != nil {
		s.Fatal("Failed to create mount point: ", err)
	}
	if err := testexec.CommandContext(shortCtx, "mount", "-o", "loop", src, dst).Run(); err != nil {
		s.Fatal("Failed to mount filesystem: ", err)
	}
	defer func() {
		s.Log("Unmounting ", dst)
		if err := testexec.CommandContext(ctx, "umount", "-f", dst).Run(); err != nil {
			s.Error("Failed to unmount filesystem: ", err)
		}
	}()

	// The message shouldn't take long to show up, so derive a short context for it.
	watchCtx, watchCancel := context.WithTimeout(shortCtx, 15*time.Second)
	defer watchCancel()

	// We expect to see a message like "[124273.844282] EXT4-fs (loop4): mounted filesystem without journal. Opts: (null)".
	const expMsg = "mounted filesystem"
	s.Logf("Watching for %q in dmesg output", expMsg)
WatchLoop:
	for {
		select {
		case msg := <-dmesgCh:
			s.Logf("Got message %q", msg)
			if strings.Contains(msg, expMsg) {
				break WatchLoop
			}
		case <-watchCtx.Done():
			s.Fatalf("Didn't see %q in dmesg: %v", expMsg, watchCtx.Err())
		}
	}
}

// makeFilesystem creates an ext4 filesystem of the requested size (in bytes) at path p.
func makeFilesystem(ctx context.Context, p string, size int64) error {
	f, err := os.Create(p)
	if err != nil {
		return err
	}

	// Clean up if we get an error mid-initialization.
	toClose := f
	defer func() {
		if toClose != nil {
			toClose.Close()
		}
	}()

	// Seek to the end of the requested size and write a byte.
	if _, err := f.Seek(size-1, 0); err != nil {
		return err
	}
	if _, err := f.Write([]byte{0xff}); err != nil {
		return err
	}
	toClose = nil // disarm cleanup
	if err := f.Close(); err != nil {
		return err
	}

	return testexec.CommandContext(ctx, "mkfs.ext4", p).Run(testexec.DumpLogOnError)
}

// streamDmesg clears the kernel ring bugger and then starts a dmesg process and
// asynchronously copies all log messages to a channel. The caller is responsible
// for killing and waiting on the returned process.
func streamDmesg(ctx context.Context) (*testexec.Cmd, <-chan string, error) {
	// Clear the buffer first so we don't see stale messages.
	if err := testexec.CommandContext(ctx, "dmesg", "--clear").Run(
		testexec.DumpLogOnError); err != nil {
		return nil, nil, errors.Wrap(err, "failed to clear log buffer")
	}

	// Start a dmesg process that writes messages to stdout as they're logged.
	cmd := testexec.CommandContext(ctx, "dmesg", "--follow")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start dmesg")
	}

	// Start a goroutine that just passes lines from dmesg to a channel.
	ch := make(chan string)
	go func() {
		defer close(ch)

		// Writes msg to ch and returns true if more messages should be written.
		writeMsg := func(msg string) bool {
			// To avoid blocking forever on a write to ch if nobody's reading from
			// it, we use a non-blocking write. If the channel isn't writable, sleep
			// briefly and then check if the context's deadline has been reached.
			for {
				if ctx.Err() != nil {
					return false
				}

				select {
				case ch <- msg:
					return true
				default:
					testing.Sleep(ctx, 10*time.Millisecond)
				}
			}
		}

		// The Scan method will return false once the dmesg process is killed.
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if !writeMsg(sc.Text()) {
				break
			}
		}
		// Don't bother checking sc.Err(). The test will already fail if the expected
		// message isn't seen.
	}()

	return cmd, ch, nil
}

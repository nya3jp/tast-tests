// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"compress/gzip"
	"context"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/chrome/internal/setup"
	"chromiumos/tast/testing"
)

// ExtensionBackgroundPageURL returns the URL to the background page for
// the extension with the supplied ID.
func ExtensionBackgroundPageURL(extID string) string {
	return extension.BackgroundPageURL(extID)
}

// ComputeExtensionID computes the 32-character ID that Chrome will use for an unpacked
// extension in dir. If the extension's manifest file contains a public key, it is hashed
// into the ID; otherwise the directory name is hashed.
func ComputeExtensionID(dir string) (string, error) {
	return extension.ComputeExtensionID(dir)
}

// AddTastLibrary introduces tast library into the page for the given conn.
// This introduces a variable named "tast" to its scope, and it is the
// caller's responsibility to avoid the conflict.
func AddTastLibrary(ctx context.Context, conn *Conn) error {
	// Ensure the page is loaded so the tast library will be added properly.
	if err := conn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return errors.Wrap(err, "failed waiting for page to load")
	}
	return conn.Eval(ctx, extension.TastLibraryJS, nil)
}

// SaveTraceToFile marshals the given trace into a binary protobuf and saves it
// to a gzip archive at the specified path.
func SaveTraceToFile(ctx context.Context, trace *trace.Trace, path string) error {
	data, err := proto.Marshal(trace)
	if err != nil {
		return errors.Wrap(err, "could not marshal trace to binary")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrap(err, "could not open file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close file: ", err)
		}
	}()

	writer := gzip.NewWriter(file)
	defer func() {
		if err := writer.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close gzip writer: ", err)
		}
	}()

	if _, err := writer.Write(data); err != nil {
		return errors.Wrap(err, "could not write the data")
	}

	if err := writer.Flush(); err != nil {
		return errors.Wrap(err, "could not flush the gzip writer")
	}

	return nil
}

// PrepareForRestart prepares for Chrome restart.
//
// This function removes a debugging port file for a current Chrome process.
// By calling this function before purposefully restarting Chrome, you can
// reliably connect to a new Chrome process without accidentally connecting to
// an old Chrome process by a race condition.
func PrepareForRestart() error {
	return setup.PrepareForRestart()
}

// moveUserCrashDumps copies the contents of the user crash directory to the
// system crash directory.
func moveUserCrashDumps() error {
	// Normally user crashes are written to /home/user/(hash)/crash as they
	// contain PII, but for test images they are written to /home/chronos/crash.
	// https://crrev.com/c/1986701
	const (
		userCrashDir   = "/home/chronos/crash"
		systemCrashDir = "/var/spool/crash"
		crashGroup     = "crash-access"
	)

	g, err := user.LookupGroup(crashGroup)
	if err != nil {
		return err
	}
	gid, err := strconv.ParseInt(g.Gid, 10, 32)
	if err != nil {
		return errors.Wrapf(err, "failed to parse gid %q", g.Gid)
	}

	if err := os.MkdirAll(systemCrashDir, 02770); err != nil {
		return err
	}

	if err := os.Chown(systemCrashDir, 0, int(gid)); err != nil {
		return err
	}

	fis, err := ioutil.ReadDir(userCrashDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, fi := range fis {
		if !fi.Mode().IsRegular() {
			continue
		}

		// As they are in different partitions, os.Rename() doesn't work.
		src := filepath.Join(userCrashDir, fi.Name())
		dst := filepath.Join(systemCrashDir, fi.Name())
		if err := fsutil.MoveFile(src, dst); err != nil {
			return err
		}

		if err := os.Chown(dst, 0, int(gid)); err != nil {
			return err
		}
	}

	return nil
}

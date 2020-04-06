// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileutil provides utilities for operating files in remote wifi tests.
package fileutil

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// WriteToHost writes the content to a tmp file and uploads to the given host.
// TODO(crbug.com/1019537): replace this if similar function is provided in SSH utilities.
func WriteToHost(ctx context.Context, hst *ssh.Conn, path string, data []byte) error {
	tmpfile, err := ioutil.TempFile("", "upload_tmp_")
	if err != nil {
		return errors.Wrap(err, "unable to create temp file")
	}
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(data); err != nil {
		tmpfile.Close()
		return errors.Wrap(err, "unable to write to temp file")
	}
	tmpfile.Close()

	pathMap := map[string]string{tmpfile.Name(): path}
	if _, err := linuxssh.PutFiles(ctx, hst, pathMap, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "unable to upload file to host")
	}
	return nil
}

// WriteToHostDirect writes content directly to a remote path of given host without trying
// to unlink the old file. WriteToHost() does not work when operating on sysfs because it
// uses linuxssh.PutFiles() and the method will uncompress the compressed content, which
// invokes a unlink to the target file, and it is illegal on procfs/sysfs.
func WriteToHostDirect(ctx context.Context, host *ssh.Conn, path string, content []byte) error {
	cmd := host.Command("sh", "-c", fmt.Sprintf("cat > %s", shutil.Escape(path)))

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	cmd.Stdin = bytes.NewReader(content)

	if err := cmd.Run(ctx); err != nil {
		return errors.Wrapf(err, "command failed with stderr %q", string(stderrBuf.Bytes()))
	}
	return nil
}

// PrepareOutDirFile prepares the base directory of the path under OutDir and opens the file.
func PrepareOutDirFile(ctx context.Context, filename string) (*os.File, error) {
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get OutDir")
	}
	filepath := path.Join(outdir, filename)
	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create basedir for %q", filepath)
	}
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file %q", filepath)
	}
	return f, nil
}

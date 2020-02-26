// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileutil provides utilities for operating files in remote wifi tests.
package fileutil

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// WriteToHost writes the content to a tmp file and uploads to the given host.
// TODO(crbug.com/1019537): replace this if similar function is provided in SSH utilities.
func WriteToHost(ctx context.Context, hst *host.SSH, path string, data []byte) error {
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
func WriteToHostDirect(ctx context.Context, host *host.SSH, path string, content []byte) error {
	cmd := host.Command("tee", path)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get StdinPipe")
	}
	var buf bytes.Buffer
	cmd.Stderr = &buf

	if err := cmd.Start(ctx); err != nil {
		pipe.Close()
		return errors.Wrap(err, "failed to run command")
	}
	if _, err := pipe.Write(content); err != nil {
		pipe.Close()
		cmd.Abort()
		cmd.Wait(ctx)
		return errors.Wrap(err, "failed to write content")
	}
	pipe.Close()
	if err := cmd.Wait(ctx); err != nil {
		return errors.Wrapf(err, "command failed with stderr %q", string(buf.Bytes()))
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

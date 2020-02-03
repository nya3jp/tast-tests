// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileutils provides utilities for operating files in remote wifi tests.
package fileutils

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/testing"
)

// WriteToHost writes the content to a tmp file and uploads to the given host.
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
	if _, err := hst.PutFiles(ctx, pathMap, host.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "unable to upload file to host")
	}
	return nil
}

func prepareOutDirFile(ctx context.Context, filename string) (*os.File, error) {
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

// ReadToOutDir spawns a goroutine which reads from a reader and collects the output to a file in test result dir.
func ReadToOutDir(ctx context.Context, filename string, reader io.ReadCloser) error {
	f, err := prepareOutDirFile(ctx, filename)
	if err != nil {
		return err
	}
	go func() {
		defer reader.Close()
		defer f.Close()
		io.Copy(f, reader)
	}()
	return nil
}

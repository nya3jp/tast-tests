// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileutils provides utilities for operating files in remote wifi tests.
package fileutils

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/network/commander"
	"chromiumos/tast/testing"
)

// WriteToHost writes the content to a tmp file and uploads to the given host.
func WriteToHost(ctx context.Context, hst *host.SSH, path string, data []byte) error {
	tmpfile, err := ioutil.TempFile("/tmp", "upload_tmp")
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
		return nil, errors.New("do not support OutDir")
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
func ReadToOutDir(ctx context.Context, filename string, reader io.ReadCloser, prefix []byte) error {
	f, err := prepareOutDirFile(ctx, filename)
	if err != nil {
		return err
	}
	if _, err := f.Write(prefix); err != nil {
		f.Close()
		return errors.Wrap(err, "failed to write log prefix")
	}
	go func() {
		defer reader.Close()
		io.Copy(f, reader)
		f.Close()
	}()
	return nil
}

// SyncTime syncs the time on local (workstation) and host.
func SyncTime(ctx context.Context, host commander.Commander) error {
	const nsecBase = 1e9
	t := time.Now().UnixNano()
	timeStr := fmt.Sprintf("%d.%09d", t/nsecBase, t%nsecBase)
	if host.Command("date", "-u", fmt.Sprintf("--set=@%s", timeStr)).Run(ctx) == nil {
		return nil
	}
	// Retry with Busybox format "%Y%m%d%H%M.%S".
	const format = "200601021504.05"
	bbTime := time.Now().Format(format)
	return host.Command("data", "-u", bbTime).Run(ctx)
}

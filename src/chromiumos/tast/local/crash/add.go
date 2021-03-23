// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
)

// AddFakeMinidumpCrash adds a fake minidump crash entry to crash.SystemCrashDir and returns a
// SendData expected to be reported by crash_sender when it processes the entry.
func AddFakeMinidumpCrash(ctx context.Context, basename string) (expected *SendData, err error) {
	return addFakeCrash(ctx, basename, ".dmp", "minidump")
}

// AddFakeKernelCrash adds a fake kernel crash entry to crash.SystemCrashDir and returns a
// SendData expected to be reported by crash_sender when it processes the entry.
func AddFakeKernelCrash(ctx context.Context, basename string) (expected *SendData, err error) {
	return addFakeCrash(ctx, basename, ".kcrash", "kcrash")
}

func addFakeCrash(ctx context.Context, basename, payloadExt, payloadKind string) (expected *SendData, err error) {
	const (
		executable  = "some_exec"
		version     = "some_version"
		payloadSize = 1024 * 1024
	)
	metaPath := filepath.Join(SystemCrashDir, basename+".meta")
	payloadPath := filepath.Join(SystemCrashDir, basename+payloadExt)

	// Create a payload file with random bytes. Since crash_sender counts bytes
	// for the rate limit after compressing the payload, we won't hit the rate
	// limit with naive zeroed files.
	if err := CreateRandomFile(payloadPath, payloadSize); err != nil {
		return nil, err
	}
	meta := fmt.Sprintf("exec_name=%s\nver=%s\npayload=%s\ndone=1\n", executable, version, filepath.Base(payloadPath))
	if err := ioutil.WriteFile(metaPath, []byte(meta), 0644); err != nil {
		return nil, err
	}
	return expectedSendData(ctx, metaPath, payloadPath, payloadKind, version, executable)
}

// CreateRandomFile creates a file at the given path, of |size| bytes and random contents.
func CreateRandomFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	_, copyErr := io.CopyN(f, rand.Reader, size)
	closeErr := f.Close()

	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func expectedSendData(ctx context.Context, metadataPath, payloadPath, payloadKind, version, executable string) (*SendData, error) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /etc/lsb-release")
	}
	board := lsb[lsbrelease.Board]

	// On some devices like betty crossystem will fail. Fall back to "undefined" in such cases.
	out, _ := testexec.CommandContext(ctx, "crossystem", "hwid").Output()
	hwid := string(out)
	if hwid == "" {
		hwid = "undefined"
	}

	exp := &SendData{
		MetadataPath: metadataPath,
		PayloadPath:  payloadPath,
		PayloadKind:  payloadKind,
		Product:      "ChromeOS",
		Version:      version,
		Board:        board,
		HWClass:      hwid,
		Executable:   executable,
	}
	return exp, nil
}

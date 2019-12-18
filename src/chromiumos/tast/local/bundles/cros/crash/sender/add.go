// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sender

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/lsbrelease"
)

// AddFakeMinidumpCrash adds a fake minidump crash entry to dir and returns a
// SendData expected to be reported by crash_sender when it processes the entry.
func AddFakeMinidumpCrash(ctx context.Context, dir, basename string) (expected *SendData, err error) {
	const (
		executable = "some_exec"
		version    = "some_version"
	)
	metaPath := filepath.Join(dir, basename+".meta")
	dmpPath := filepath.Join(dir, basename+".dmp")

	if err := ioutil.WriteFile(dmpPath, nil, 0644); err != nil {
		return nil, err
	}
	meta := fmt.Sprintf("exec_name=%s\nver=%s\npayload=%s\ndone=1\n", executable, version, filepath.Base(dmpPath))
	if err := ioutil.WriteFile(metaPath, []byte(meta), 0644); err != nil {
		return nil, err
	}
	return expectedSendData(ctx, metaPath, dmpPath, "minidump", version, executable)
}

// AddFakeKernelCrash adds a fake kernel crash entry to dir and returns a
// SendData expected to be reported by crash_sender when it processes the entry.
func AddFakeKernelCrash(ctx context.Context, dir, basename string) (expected *SendData, err error) {
	const (
		executable = "kernel"
		version    = "some_version"
	)
	metaPath := filepath.Join(dir, basename+".meta")
	kcrashPath := filepath.Join(dir, basename+".kcrash")

	if err := ioutil.WriteFile(kcrashPath, nil, 0644); err != nil {
		return nil, err
	}
	meta := fmt.Sprintf("exec_name=%s\nver=%s\npayload=%s\ndone=1\n", executable, version, filepath.Base(kcrashPath))
	if err := ioutil.WriteFile(metaPath, []byte(meta), 0644); err != nil {
		return nil, err
	}
	return expectedSendData(ctx, metaPath, kcrashPath, "kcrash", version, executable)
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

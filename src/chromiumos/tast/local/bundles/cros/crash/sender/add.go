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
	return addFakeCrash(ctx, dir, basename, ".dmp", "minidump")
}

// AddFakeKernelCrash adds a fake kernel crash entry to dir and returns a
// SendData expected to be reported by crash_sender when it processes the entry.
func AddFakeKernelCrash(ctx context.Context, dir, basename string) (expected *SendData, err error) {
	return addFakeCrash(ctx, dir, basename, ".kcrash", "kcrash")
}

func addFakeCrash(ctx context.Context, dir, basename, payloadExt, payloadKind string) (expected *SendData, err error) {
	const (
		executable = "some_exec"
		version    = "some_version"
	)
	metaPath := filepath.Join(dir, basename+".meta")
	payloadPath := filepath.Join(dir, basename+payloadExt)

	if err := ioutil.WriteFile(payloadPath, nil, 0644); err != nil {
		return nil, err
	}
	meta := fmt.Sprintf("exec_name=%s\nver=%s\npayload=%s\ndone=1\n", executable, version, filepath.Base(payloadPath))
	if err := ioutil.WriteFile(metaPath, []byte(meta), 0644); err != nil {
		return nil, err
	}
	return expectedSendData(ctx, metaPath, payloadPath, payloadKind, version, executable)
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

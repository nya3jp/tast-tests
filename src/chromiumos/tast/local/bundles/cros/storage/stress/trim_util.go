// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// 0x1277 is ioctl BLKDISCARD command.
	blkdiscardIoctlCommand = 0x1277

	// IOCTL error number for "Trim not supported".
	ioctlNotSupportedError = 95

	// TrimChunkSize is the size (in bytes) of a data chunk to trim.
	TrimChunkSize = 192 * 1024
)

type trimArgs struct {
	offset, size uint64
}

// RunTrim invokes ioctl trim command.
func RunTrim(f *os.File, offset, size uint64) error {
	_, _, errCode := syscall.RawSyscall(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		blkdiscardIoctlCommand,
		uintptr(unsafe.Pointer(&trimArgs{offset: offset * TrimChunkSize, size: size})))
	if errCode == ioctlNotSupportedError {
		return errors.New("IOCTL trim command not supported")
	} else if errCode != 0 {
		return errors.Errorf("failed to execute trim command: %d", errCode)
	}
	return nil
}

// CalculateZeroOneHashes calculates hash values for zero'ed and one'ed data.
func CalculateZeroOneHashes(ctx context.Context) (zeroHash, oneHash string) {
	template := "dd if=/dev/zero bs=%d count=1 status=none | tr '\\0' '%s' | sha256sum | cut -d\" \" -f 1"

	out, err := testexec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf(template, TrimChunkSize, "\\0")).CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "Failed running: ", template)
		return "", ""
	}
	zeroHash = strings.TrimSpace(string(out))

	out, err = testexec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf(template, TrimChunkSize, "\\xff")).CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "Failed running: ", template)
		return "", ""
	}
	oneHash = strings.TrimSpace(string(out))

	return zeroHash, oneHash
}

// CalculateCurrentHashes calculates hash values for all chunks of the given file.
func CalculateCurrentHashes(ctx context.Context, filename string, chunkCount uint64) (hashes []string, err error) {
	for i := uint64(0); i < chunkCount; i++ {
		cmd := fmt.Sprintf("dd if=%s of=/dev/stdout bs=%d count=1 skip=%d iflag=direct | sha256sum | cut -d\" \" -f 1",
			filename, TrimChunkSize, i)
		out, err := testexec.CommandContext(ctx, "bash", "-c", cmd).Output(testexec.DumpLogOnError)
		if err != nil {
			return hashes, errors.Wrapf(err, " failed running command: %s", cmd)
		}
		hashes = append(hashes, strings.TrimSpace(string(out)))
	}
	return hashes, nil
}

// WriteRandomData dumps random data to a given file/disk.
func WriteRandomData(ctx context.Context, filename string, chunkCount uint64) error {
	cmd := fmt.Sprintf("dd if=/dev/urandom of=%s bs=%d count=%d oflag=direct", filename, TrimChunkSize, chunkCount)
	if err := testexec.CommandContext(ctx, "bash", "-c", cmd).Run(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "Failed running command: ", cmd, ", error: ", err)
		return err
	}
	return nil
}

// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
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
	_, _, errCode := unix.RawSyscall(
		unix.SYS_IOCTL,
		uintptr(f.Fd()),
		blkdiscardIoctlCommand,
		uintptr(unsafe.Pointer(&trimArgs{offset: offset * TrimChunkSize, size: size})))
	if errCode == ioctlNotSupportedError {
		return errors.New("IOCTL trim command not supported")
	}
	if errCode != 0 {
		return errors.Errorf("failed to execute trim command: %d", errCode)
	}
	return nil
}

// ZeroHash calculates sha256 has of the 0-value block.
func ZeroHash() string {
	buf := make([]byte, TrimChunkSize)
	return sha256String(buf)
}

// OneHash calculates sha256 has of the 1-value block.
func OneHash() string {
	buf := make([]byte, TrimChunkSize)
	for i := range buf {
		buf[i] = 0xff
	}
	return sha256String(buf)
}

// CalculateCurrentHashes calculates hash values for all chunks of the given file.
func CalculateCurrentHashes(filename string, chunkCount uint64) (hashes []string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return hashes, errors.Wrapf(err, " failed opening device: %s", filename)
	}
	defer f.Close()

	buf := make([]byte, TrimChunkSize)

	for i := uint64(0); i < chunkCount; i++ {
		if n, err := f.ReadAt(buf, int64(i*TrimChunkSize)); err != nil || n != TrimChunkSize {
			return hashes, errors.Wrapf(err, " error reading from: %s", filename)
		}
		hashes = append(hashes, sha256String(buf))
	}
	return hashes, nil
}

// WriteRandomData dumps random data to a given file/disk.
func WriteRandomData(filename string, chunkCount uint64) error {
	f, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return errors.Wrapf(err, " failed opening device: %s", filename)
	}
	defer f.Close()

	// Prepare a random buffer to write.
	buf := make([]byte, TrimChunkSize)
	if n, err := rand.Read(buf); err != nil || uint64(n) != TrimChunkSize {
		return errors.Wrapf(err, " error generating random buffer of length: %d", TrimChunkSize)
	}

	// Write the disk partition in chunks of TrimChunkSize bytes.
	for i := uint64(0); i < chunkCount; i++ {
		if n, err := f.WriteAt(buf, int64(i*TrimChunkSize)); err != nil || uint64(n) != TrimChunkSize {
			return errors.Wrapf(err, " error writing random data to: %s", filename)
		}
	}
	return nil
}

func sha256String(buf []byte) string {
	sha := sha256.Sum256(buf)
	return hex.EncodeToString(sha[:])
}

func random(min, max int) int {
	return rand.Intn(max-min) + min
}

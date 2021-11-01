// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package intelfwextractor extracts the fw dump and validate its contents.
package intelfwextractor

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
)

const (
	yoyoMagic  = 0x14789633 // HrP2, CcP2, JfP2, ThP2
	validMagic = 0x14789632 // StP2
)

// DevcoreHeader is a struct that contains the fw dump header information.
type DevcoreHeader struct {
	Magic    uint32
	FileSize uint32
}

// ValidateFWDump extracts and validate the contents of the fw dump.
func ValidateFWDump(ctx context.Context, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return errors.Wrapf(err, "failed to open the file %s: ", file)
	}
	defer f.Close()

	var r io.ReadCloser
	r, err = gzip.NewReader(f)
	if err != nil {
		return errors.Wrapf(err, "failed to create gzip reader for %s: ", f)
	}
	defer r.Close()

	fwDumpData, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.Wrap(err, "failed to read the fw_dump")
	}

	fwDumpBuffer := bytes.NewBuffer(fwDumpData)

	// Validate the header
	fwDumpHeader := DevcoreHeader{}
	err = binary.Read(fwDumpBuffer, binary.LittleEndian, &fwDumpHeader)
	if err != nil {
		return errors.Wrap(err, "failed to read the header of the fw dump")
	}

	// Check the fw dump size.
	if len(fwDumpData) != int(fwDumpHeader.FileSize) {
		return errors.Errorf("unexpected file size: got %d, want %d", len(fwDumpData), fwDumpHeader.FileSize)
	}

	// Check the magic signature of the fw dump. The following are the expected signatures.
	if (fwDumpHeader.Magic != yoyoMagic) && (fwDumpHeader.Magic != validMagic) {
		return errors.Errorf("unexpected magic signature: got %x, want {%x, %x}", fwDumpHeader.Magic, validMagic, yoyoMagic)
	}

	return nil
}

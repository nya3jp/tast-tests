// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Intel License:

/*
# SPDX-License-Identifier: BSD-3-Clause
#
# Copyright (C) 2012-2014, 2018-2020 Intel Corporation
# Copyright (C) 2013-2014 Intel Mobile Communications GmbH
# Copyright (C) 2015 Intel Deutschland GmbH
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
#
# Redistributions of source code must retain the above copyright
# notice, this list of conditions and the following disclaimer.
# Redistributions in binary form must reproduce the above copyright
# notice, this list of conditions and the following disclaimer in
# the documentation and/or other materials provided with the distribution.
# Neither the name Intel Corporation nor the names of its
# contributors may be used to endorse or promote products derived
# from this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
# "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
# LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
# A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
# OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
# SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
# LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
# DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
# THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
# OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Package intelfwextractor extracts the fw dump and validate its contents.
package intelfwextractor

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	yoyoMagic  = 0x14789633
	validMagic = 0x14789632
)

// DevcoreHeader is a struct that contains the fw dump header information.
type DevcoreHeader struct {
	Magic    uint32
	FileSize uint32
}

// ValidateFWDump extracts and validate the contents of the fw dump.
func ValidateFWDump(ctx context.Context, file string) error {
	devcoreFile := strings.Replace(file, ".devcore.gz", ".devcore", 1)
	logFile := strings.Replace(file, ".devcore.gz", ".log", 1)
	metaFile := strings.Replace(file, ".devcore.gz", ".meta", 1)
	generatedFiles := []string{devcoreFile, logFile, metaFile}

	if err := testexec.CommandContext(ctx, "gzip", "-dk", file).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to decommpress the file %s", file)
	}
	defer func() error {
		// Remove generated files.
		for _, f := range generatedFiles {
			if err := os.Remove(f); err != nil {
				return errors.Wrapf(err, "failed to remove the file %s", f)
			}
		}
		return nil
	}()

	// Read the entire file into memory.
	data, err := ioutil.ReadFile(devcoreFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read the file %s:", devcoreFile)
	}
	buffer := bytes.NewBuffer(data)

	// Validate the header
	header := DevcoreHeader{}
	err = binary.Read(buffer, binary.LittleEndian, &header)
	if err != nil {
		return errors.Wrap(err, "failed to read the header of the fw dump")
	}

	// Check the fw dump size.
	if len(data) != int(header.FileSize) {
		return errors.Errorf("unexpected file size: got %d, want %d", len(data), header.FileSize)
	}

	// Check the magic signature of the dump.
	if (header.Magic != yoyoMagic) && (header.Magic != validMagic) {
		return errors.Errorf("unexpected magic signature: got %x, want {%x, %x}", header.Magic, validMagic, yoyoMagic)
	}

	return nil
}

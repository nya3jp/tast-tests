// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package files

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

// NOTE: While AES, SHA256, and PBKDF2 are used here, it is not for security,
// and is only used for speedy generation of test data that can be reproduced.

// AppendFile appends to the the file at path, for a size of length bytes.
// The data written is generated with AES.
// length should always be multples of 32 bytes.
// Note that key and path should not contain single quote (').
// Note that a block size of 32 is used so that we can test partial pages/sector,
// while having acceptable performance. (bs=1 can be slow.)
func AppendFile(ctx context.Context, runner hwsec.CmdRunner, path, key string, length int) error {
	if length%32 != 0 {
		return errors.Errorf("length %d not multiples of 32", length)
	}
	length32 := length / 32

	cmd := fmt.Sprintf("dd if=/dev/zero bs=32 count=%d | openssl enc -aes-128-ctr -pass pass:'%s' -nosalt -pbkdf2 -iter 1 >> '%s'", length32, key, path)
	if _, err := runner.Run(ctx, "sh", "-c", cmd); err != nil {
		return errors.Wrapf(err, "failed to write test file %q", path)
	}
	return nil
}

// CalcSHA256 calculates the SHA256 sum of a file on the DUT.
func CalcSHA256(ctx context.Context, runner hwsec.CmdRunner, path string) (string, error) {
	raw, err := runner.Run(ctx, "sha256sum", "-b", path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command to calculate sha256 sum for %q", path)
	}
	// Now process the output to get the sum.
	out := string(raw)
	arr := strings.Split(out, " ")
	if len(out) < 2 {
		return "", errors.Errorf("invalid output from sha256sum, missing fields: %q", out)
	}

	result := arr[0]
	if len(result) != 64 {
		return "", errors.Errorf("invalid output from sha256sum, incorrect output length: %q", out)
	}

	return result, nil
}

// ResetFile ensures that the given file exists on the DUT and is of length 0.
func ResetFile(ctx context.Context, runner hwsec.CmdRunner, path string) error {
	if _, err := runner.Run(ctx, "truncate", "--size", "0", path); err != nil {
		return errors.Wrapf(err, "failed to truncate file %q", path)
	}
	return nil
}

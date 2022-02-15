// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11test

import (
	"bufio"
	"bytes"
	"context"
	"regexp"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

// SetupP11TestToken configures a PKCS #11 token for testing using database in scratchpadPath.
// unloadUserToken specify whether to unload all user tokens and authData is the initial token authorization data.
func SetupP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string, unloadUserToken bool) error {
	CleanupP11TestToken(ctx, r, scratchpadPath)

	if unloadUserToken == true {
		lines, err := r.Run(ctx, "chaps_client", "--list")
		if err != nil {
			return errors.Wrap(err, "failed to list token path")
		}
		scanner := bufio.NewScanner(bytes.NewReader(lines))
		re := regexp.MustCompile(`Slot \d+: (/.*)$`)
		for scanner.Scan() {
			if re.Match([]byte(scanner.Text())) {
				if _, err := r.Run(ctx, "sudo", "chaps_client", "--unload", "--path="+re.FindStringSubmatch(scanner.Text())[1]); err != nil {
					return errors.Wrap(err, "failed to unload scratchpad")
				}
			}
		}
	}

	if _, err := r.Run(ctx, "mkdir", "-p", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to create scratchpad")
	}

	if _, err := r.Run(ctx, "chown", "chaps:chronos-access", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to change owner")

	}

	if _, err := r.Run(ctx, "chmod", "750", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to change mode")
	}

	return nil
}

// LoadP11TestToken loads the test token onto a slot stored in scratchpadPath.
func LoadP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath, authData string) error {

	if _, err := r.Run(ctx, "sudo", "chaps_client", "--load", "--path="+scratchpadPath, "--auth="+authData); err != nil {
		return errors.Wrap(err, "failed to load PKCS11 token")
	}

	return nil
}

// UnloadP11TestToken unloads loaded test token stored in scratchpadPath.
func UnloadP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {

	if _, err := r.Run(ctx, "sudo", "chaps_client", "--unload", "--path="+scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to unload PKCS11 token")
	}

	return nil
}

// ChangeP11TestTokenAuthData changes authorization data auth_data by new_auth_data stored in scratchpadPath.
func ChangeP11TestTokenAuthData(ctx context.Context, r hwsec.CmdRunner, scratchpadPath, authData, newAuthData string) error {

	if _, err := r.Run(ctx, "sudo", "chaps_client", "--load", "--path="+scratchpadPath, "--auth="+authData, "--new_auth="+newAuthData); err != nil {
		return errors.Wrap(err, "failed to change PKCS11 token auth data")
	}

	return nil
}

// CleanupP11TestToken deletes the test token stored in scratchpadPath.
func CleanupP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	UnloadP11TestToken(ctx, r, scratchpadPath)
	if _, err := r.Run(ctx, "rm", "-rf", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to remove the scratchpad directory")
	}
	return nil
}

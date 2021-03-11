// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
)

// tpmManagerBinary is used to interact with the tpm_managerd process over
// 'tpm_manager_client' executable. For more details of the arguments of the
// functions in this file, please check //src/platform2/tpm_manager/client/main.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned main.cc.
type tpmManagerBinary struct {
	runner CmdRunner
}

// newTPMManagerBinary is a factory function to create a
// tpmManagerBinary instance.
func newTPMManagerBinary(r CmdRunner) *tpmManagerBinary {
	return &tpmManagerBinary{r}
}

// call is a simple utility that helps to call tpm_manager_client.
func (c *tpmManagerBinary) call(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Run(ctx, "tpm_manager_client", args...)
}

// defineSpace calls "tpm_manager_client define_space".
func (c *tpmManagerBinary) defineSpace(ctx context.Context, size int, bindToPCR0 bool, index, attributes, password string) ([]byte, error) {
	args := []string{"define_space", "--index=" + index, "--size=" + strconv.Itoa(size)}
	if bindToPCR0 {
		args = append(args, "--bind_to_pcr0")
	}
	if attributes != "" {
		args = append(args, "--attributes="+attributes)
	}
	if password != "" {
		args = append(args, "--password="+password)
	}
	return c.call(ctx, args...)
}

// destroySpace calls "tpm_manager_client destroy_space".
func (c *tpmManagerBinary) destroySpace(ctx context.Context, index string) ([]byte, error) {
	return c.call(ctx, "destroy_space", "--index="+index)
}

// writeSpace calls "tpm_manager_client write_space".
func (c *tpmManagerBinary) writeSpace(ctx context.Context, index, file, password string) ([]byte, error) {
	args := []string{"write_space", "--index=" + index, "--file=" + file}
	if password != "" {
		args = append(args, "--password="+password)
	}
	return c.call(ctx, args...)
}

// readSpace calls "tpm_manager_client read_space".
func (c *tpmManagerBinary) readSpace(ctx context.Context, index, file, password string) ([]byte, error) {
	args := []string{"read_space", "--index=" + index, "--file=" + file}
	if password != "" {
		args = append(args, "--password="+password)
	}
	return c.call(ctx, args...)
}

// getDAInfo calls "tpm_manager_client get_da_info".
func (c *tpmManagerBinary) getDAInfo(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "get_da_info")
}

// resetDALock calls "tpm_manager_client reset_da_lock".
func (c *tpmManagerBinary) resetDALock(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "reset_da_lock")
}

// takeOwnership calls "tpm_manager_client take_ownership".
func (c *tpmManagerBinary) takeOwnership(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "take_ownership")
}

// status calls "tpm_manager_client status".
func (c *tpmManagerBinary) status(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "status")
}

// nonsensitiveStatus calls "tpm_manager_client status --nonsensitive".
func (c *tpmManagerBinary) nonsensitiveStatus(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "status", "--nonsensitive")
}

// nonsensitiveStatusIgnoreCache calls "tpm_manager_client status --nonsensitive --ignore_cache".
func (c *tpmManagerBinary) nonsensitiveStatusIgnoreCache(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "status", "--nonsensitive", "--ignore_cache")
}

// clearOwnerPassword calls "tpm_manager_client clear_owner_password".
func (c *tpmManagerBinary) clearOwnerPassword(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "clear_owner_password")
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
)

// TpmManagerBinary is used to interact with the tpm_managerd process over
// 'tpm_manager_client' executable. For more details of the arguments of the
// functions in this file, please check //src/platform2/tpm_manager/client/main.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned main.cc.
type TpmManagerBinary struct {
	runner CmdRunner
}

// NewTpmManagerBinary is a factory function to create a
// TpmManagerBinary instance.
func NewTpmManagerBinary(r CmdRunner) (*TpmManagerBinary, error) {
	return &TpmManagerBinary{r}, nil
}

// call is a simple utility that helps to call tpm_manager_client.
func (c *TpmManagerBinary) call(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Run(ctx, "tpm_manager_client", args...)
}

// DefineSpace calls "tpm_manager_client define_space".
func (c *TpmManagerBinary) DefineSpace(ctx context.Context, size int, bindToPCR0 bool, index, attributes, password string) ([]byte, error) {
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

// DestroySpace calls "tpm_manager_client destroy_space".
func (c *TpmManagerBinary) DestroySpace(ctx context.Context, index string) ([]byte, error) {
	return c.call(ctx, "destroy_space", "--index="+index)
}

// WriteSpace calls "tpm_manager_client write_space".
func (c *TpmManagerBinary) WriteSpace(ctx context.Context, index, file, password string) ([]byte, error) {
	args := []string{"write_space", "--index=" + index, "--file=" + file}
	if password != "" {
		args = append(args, "--password="+password)
	}
	return c.call(ctx, args...)
}

// GetDAInfo calls "tpm_manager_client get_da_info".
func (c *TpmManagerBinary) GetDAInfo(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "get_da_info")
}

// ResetDALock calls "tpm_manager_client reset_da_lock".
func (c *TpmManagerBinary) ResetDALock(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "reset_da_lock")
}

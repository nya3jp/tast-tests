// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package files contains the utilities for testing file based storage.
// It is primarily used to test cryptohomed and user's home directory.
package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
	"math/rand"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

const (
	// seedSize determine the size of the seed for FileInfo
	seedSize = 16

	// maxIterationData is the maximum amount of data to write in an iteration.
	// It is denoted in multiples of 32 bytes.
	maxIterationData = 327680 // 10MiB

	// minIterationData is the minimum amount of data to write in an iteration.
	// It is denoted in multiples of 32 bytes.
	minIterationData = 1024 // 32 KiB
)

// FileInfo stores the information related to a file under test that is stored in the DUT.
// With the information
type FileInfo struct {
	// path is the path to the file under test.
	// It should not be changed after the structure is created.
	path string

	// hash is the crypto hash object representing the state of the file on disk.
	// At all times, we can get the Sum() from the hash to verify the state of the file.
	// It's always an SHA256 hash object.
	hash *hash.Hash

	// iteration is how many times was the file modified?
	iteration int

	// runner is required to run commands on the DUT.
	runner hwsec.CmdRunner

	// seed is the seed to deterministically determine the content and size of
	// all contents written to the test file. It is generated at the initialization
	// time of this structure.
	seed []byte
}

// NewFileInfo create a new FileInfo for testing a file.
// Note that calling this method only initializes the data structures and doesn't
// touch anything on disk. To reset the on disk state, call Clear().
func NewFileInfo(ctx context.Context, path string, runner hwsec.CmdRunner) (*FileInfo, error) {
	seed := make([]byte, seedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, errors.Wrap(err, "failed to generate seed for NewFileInfo")
	}

	h := sha256.New()
	return &FileInfo{
		path:      path,
		hash:      &h,
		iteration: 0,
		runner:    runner,
		seed:      seed,
	}, nil
}

// Clear resets the file and the state in data structure back to an empty file.
func (f *FileInfo) Clear(ctx context.Context) error {
	if err := ResetFile(ctx, f.runner, f.path); err != nil {
		return errors.Wrapf(err, "failed to reset file %q", f.path)
	}
	f.iteration = 0
	h := sha256.New()
	f.hash = &h
	return nil
}

// Step append to the test file.
func (f *FileInfo) Step(ctx context.Context) error {
	// Update the file first.
	l := f.getIterationLength(f.iteration)
	k := f.getIterationKey(f.iteration)
	if err := AppendFile(ctx, f.runner, f.path, k, l); err != nil {
		return errors.Wrapf(err, "failed to append file in iteration %d", f.iteration)
	}

	// Then update the internal state.
	if err := UpdateHashForIteration(f.hash, k, l); err != nil {
		return errors.Wrapf(err, "failed to update internal state in iteration %d", f.iteration)
	}

	// Update the iteration.
	f.iteration++

	// Now run verification once to make sure it's correct.
	if err := f.Verify(ctx); err != nil {
		return errors.Wrap(err, "test files are incorrect right after Step()")
	}

	return nil
}

// Verify tests the file on disk is correct.
func (f *FileInfo) Verify(ctx context.Context) error {
	// First get the hash from the internal state.
	hInt := strings.ToLower(hex.EncodeToString((*f.hash).Sum(nil)))

	// Then get the hash from the DUT.
	hDut, err := CalcSHA256(ctx, f.runner, f.path)
	if err != nil {
		return errors.Wrap(err, "failed to calculate SHA256 on the DUT")
	}

	hDut = strings.ToLower(hDut)

	if hInt != hDut {
		// Something is wrong.
		return errors.Errorf("mismatch in SHA256 for test file %q, got %q expected %q", f.path, hDut, hInt)
	}

	return nil
}

// getIterationKey returns the key used to generate the test data for a given
// round/iteration.
func (f *FileInfo) getIterationKey(iteration int) string {
	s := hex.EncodeToString(f.seed)
	return fmt.Sprintf("%s%04d", s, iteration)
}

// getIterationLength returns the amount of data we'll write to the test file in iteration.
// The returned int is the number of bytes to write, it is always in multiples of 32.
func (f *FileInfo) getIterationLength(iteration int) int {
	logRange := math.Log(float64(maxIterationData) / float64(minIterationData))
	r := rand.New(rand.NewSource(f.getIterationSeed(iteration)))
	exp := logRange * r.Float64()
	resultFloat := float64(minIterationData) * math.Exp(exp)
	result := int(math.Round(resultFloat)) * 32
	return result
}

// getIterationSeed returns the seed used to generate data such as size
func (f *FileInfo) getIterationSeed(iteration int) int64 {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s%04x", hex.EncodeToString(f.seed), iteration)))
	var seed int64
	buf := bytes.NewReader(h[:])
	if err := binary.Read(buf, binary.LittleEndian, &seed); err != nil {
		// Shouldn't happen.
		panic("Failed to convert seed to int64, did somebody redefine sha256?")
	}
	return seed
}

// Path returns the path for the file info.
func (f *FileInfo) Path() string {
	return f.path
}

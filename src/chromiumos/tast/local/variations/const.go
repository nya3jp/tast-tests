// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package variations provides common constants of seed that can
// be used in variation smoke tests.
package variations

// pref names for controlling the variations config.
const (
	CompressedSeedPref = "variations_compressed_seed"
	SeedSignaturePref  = "variations_seed_signature"
)

// SeedData represents a variations seed, which contains information about field trials to enable on the device.
type SeedData struct {
	CompressedSeed string `json:"variations_compressed_seed"`
	SeedSignature  string `json:"variations_seed_signature"`
}

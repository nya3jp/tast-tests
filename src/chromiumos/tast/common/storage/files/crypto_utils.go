// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package files

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"hash"

	"golang.org/x/crypto/pbkdf2"

	"chromiumos/tast/errors"
)

// DeriveOpenSSLAESKeyIV derives the key and IV that was used by OpenSSL when it
// wrote the test data to the test file.
func DeriveOpenSSLAESKeyIV(key string) ([]byte, []byte) {
	// Assuming -nosalt and -iter 1, this is the parameters used in AppendFile().
	// 32 bytes because we need 16 bytes for the key and 16 bytes for the IV.
	// Note that we're assuming the usage of aes-128 in AppendFile().
	r := pbkdf2.Key([]byte(key), []byte(""), 1, 32, sha256.New)
	return r[0:16], r[16:32]
}

// UpdateHashForIteration updates the given sha256 hash object to reflect the
// contents written to the test file during the iteration/round.
// This function mirrors what AppendFile() does.
// length is the length of the data, it should be in multiples of 32 bytes.
func UpdateHashForIteration(h *hash.Hash, key string, length int) error {
	if length%32 != 0 {
		return errors.Errorf("length %d not multiples of 32", length)
	}
	length32 := length / 32

	aesKey, ctrIV := DeriveOpenSSLAESKeyIV(key)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		// Shouldn't happen.
		return errors.Wrap(err, "failed to create AES cipher in UpdateHashForIteration()")
	}

	dst := make([]byte, 32)
	src := make([]byte, 32)
	// Note that src is all zero because AppendFile() takes from /dev/zero.

	stream := cipher.NewCTR(block, ctrIV)
	for i := 0; i < length32; i++ {
		stream.XORKeyStream(dst, src)
		if _, err := (*h).Write(dst); err != nil {
			// Shouldn't happen, hashing SHA256 always succeeds.
			return errors.Wrap(err, "failed to hash data in UpdateHashForIteration()")
		}
	}

	return nil
}

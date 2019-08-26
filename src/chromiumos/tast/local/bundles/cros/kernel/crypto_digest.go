// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"reflect"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptoDigest,
		Desc: "Tests the crypto user API to compute message digests",
		Contacts: []string{
			"briannorris@chromium.org", // Original test author
			"chromeos-kernel@google.com",
			"oka@chromium.org", // Tast port author
		},
		Attr: []string{"informational"},
	})
}

func CryptoDigest(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		name string
		h    hash.Hash
	}{
		{"sha1", sha1.New()},
		{"md5", md5.New()},
		{"sha512", sha512.New()},
	} {
		const message = "This is a not-so-secret message"
		got, err := cryptoDigest(tc.name, message, tc.h.Size())
		if err != nil {
			s.Fatalf("Failed to compute digest with %s: %v", tc.name, err)
		}

		if _, err := tc.h.Write([]byte(message)); err != nil {
			s.Fatal("Failed creating expected hash: ", err)
		}
		want := tc.h.Sum(nil)

		if !reflect.DeepEqual(got, want) {
			s.Errorf("Digests were unexpectedly different for algorithm %s: got %s, want %s", tc.name,
				hex.EncodeToString(got), hex.EncodeToString(want))
		}
	}
}

// cryptoDigest computes hash of the data using kernel API with the given method.
// The size parameter tells the number of bytes the method returns.
func cryptoDigest(method, data string, size int) (_ []byte, err error) {
	sock, err := unix.Socket(unix.AF_ALG, unix.SOCK_SEQPACKET, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create socket for loading crypto module")
	}
	defer func() {
		if cerr := unix.Close(sock); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed to close socket")
		}
	}()
	if err := unix.Bind(sock, &unix.SockaddrALG{
		Type: "hash",
		Name: method,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to bind socket")
	}
	// unix.Accept does not work at this time; must invoke accept()
	// manually using unix.Syscall. https://godoc.org/golang.org/x/sys/unix#SockaddrALG
	hashFD, _, err := unix.Syscall(unix.SYS_ACCEPT, uintptr(sock), 0, 0)
	if err.(unix.Errno) != 0 {
		return nil, errors.Wrap(err, "failed on accept syscall")
	}
	defer func() {
		if cerr := unix.Close(int(hashFD)); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed to close FD used for hash computation")
		}
	}()

	// Create an io.ReadWriter for the FD. Filename doesn't matter; it's not a real file in the file system.
	h := os.NewFile(hashFD, "")
	if _, err := io.WriteString(h, data); err != nil {
		return nil, errors.Wrap(err, "failed to write data to compute hash for")
	}
	res := make([]byte, size)
	if _, err := io.ReadFull(h, res); err != nil {
		return nil, errors.Wrap(err, "failed to read hash")
	}
	return res, nil
}

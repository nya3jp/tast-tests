// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package extension

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
)

const chromeUser = "chronos" // Chrome Unix username

// ChownContentsToChrome recursively changes the ownership of the directory
// contents to the uid and gid of the Chrome's browser process.
func ChownContentsToChrome(dir string) error {
	return chownContents(dir, chromeUser)
}

// chownContents recursively chowns dir's contents to username's uid and gid.
func chownContents(dir, username string) error {
	var u *user.User
	var err error
	if u, err = user.Lookup(username); err != nil {
		return err
	}

	var uid, gid int64
	if uid, err = strconv.ParseInt(u.Uid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse uid %q", u.Uid)
	}
	if gid, err = strconv.ParseInt(u.Gid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse gid %q", u.Gid)
	}

	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, int(uid), int(gid))
	})
}

// ComputeExtensionID computes the 32-character ID that Chrome will use for an unpacked
// extension in dir. If the extension's manifest file contains a public key, it is hashed
// into the ID; otherwise the directory name is hashed.
func ComputeExtensionID(dir string) (string, error) {
	key := []byte(dir)
	mp := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(mp); !os.IsNotExist(err) {
		if k, err := readKeyFromExtensionManifest(mp); err != nil {
			return "", err
		} else if k != nil {
			key = k
		}
	}

	// Chrome computes an extension's ID by creating a SHA-256 digest of the extension's public key
	// and converting its first 16 bytes to 32 hex characters, with the added twist that the
	// characters 'a'-'p' are used rather than '0'-'f'.
	sum := sha256.Sum256(key)
	id := make([]byte, 32)
	for i, b := range sum[:len(id)/2] {
		id[i*2] = b/16 + 'a'
		id[i*2+1] = b%16 + 'a'
	}
	return string(id), nil
}

// readKeyFromExtensionManifest returns the decoded public key from an
// extension manifest located at path. An error is returned if the manifest
// is missing or malformed. A nil key is returned if the manifest is
// parsable but doesn't contain a key.
func readKeyFromExtensionManifest(path string) ([]byte, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	j := make(map[string]interface{})
	if err = json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	if enc, ok := j["key"].(string); ok {
		return base64.StdEncoding.DecodeString(enc)
	}
	return nil, nil
}

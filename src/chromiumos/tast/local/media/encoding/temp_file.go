// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encoding

import (
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
)

// CreatePublicTempFile creates a world-readable temporary file. A caller should
// close and remove the file in the end.
func CreatePublicTempFile(prefix string) (*os.File, error) {
	f, err := ioutil.TempFile("", prefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a public temporary file")
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, errors.Wrap(err, "failed to create a public temporary file")
	}
	return f, nil
}

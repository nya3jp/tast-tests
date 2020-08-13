// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"strings"
)

// CatFileLines parses file contents by line and reports the list of lines.
func (r *Reporter) CatFileLines(ctx context.Context, path string) ([]string, error) {
	res, err := r.CatFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return strings.Split(res, "\n"), nil
}

// CatFile reports the file contents as a single string.
func (r *Reporter) CatFile(ctx context.Context, path string) (string, error) {
	res, err := r.CommandOutput(ctx, "cat", path)
	if err != nil {
		return "", err
	}
	return res, nil
}

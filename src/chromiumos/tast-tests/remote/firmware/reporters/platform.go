// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

// Board reports the name of the DUT board, such as coral or veyron_minnie.
func (r *Reporter) Board(ctx context.Context) (string, error) {
	lsbContents, err := r.CatFile(ctx, "/etc/lsb-release")
	if err != nil {
		return "", errors.Wrap(err, "loading lsbrelease contents")
	}
	lsbMap, err := lsbrelease.Parse(strings.NewReader(lsbContents))
	if err != nil {
		return "", errors.Wrap(err, "parsing lsbrelease contents")
	}
	board, ok := lsbMap[lsbrelease.Board]
	if !ok {
		return "", errors.Errorf("failed to find %s in lsbrelease contents", lsbrelease.Board)
	}

	// TODO(jbettis): This should replace the code above, cros_config is more trustworthy than /etc/lsb-release
	if out, err := r.d.Conn().CommandContext(ctx, "cros_config", "/identity", "platform-name").Output(); err != nil {
		testing.ContextLogf(ctx, "cros_config failed: %s", err)
	} else {
		crosConfigBoard := strings.ToLower(string(out))
		if crosConfigBoard != board {
			testing.ContextLogf(ctx, "Board from lsb-release %q doesn't match cros_config %q", board, crosConfigBoard)
		}
	}

	return board, nil
}

// BuilderPath reports release path of the build, such as grunt-release/R93-14023.0.0.
func (r *Reporter) BuilderPath(ctx context.Context) (string, error) {
	lsbContents, err := r.CatFile(ctx, "/etc/lsb-release")
	if err != nil {
		return "", errors.Wrap(err, "loading lsbrelease contents")
	}
	lsbMap, err := lsbrelease.Parse(strings.NewReader(lsbContents))
	if err != nil {
		return "", errors.Wrap(err, "parsing lsbrelease contents")
	}
	path, ok := lsbMap[lsbrelease.BuilderPath]
	if !ok {
		return "", errors.Errorf("failed to find %s in lsbrelease contents", lsbrelease.BuilderPath)
	}
	return path, nil
}

// Model reports the name of the DUT model, such as robo360 or minnie.
func (r *Reporter) Model(ctx context.Context) (string, error) {
	return r.CommandOutput(ctx, "cros_config", "/", "name")
}

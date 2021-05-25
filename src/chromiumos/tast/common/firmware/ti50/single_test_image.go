// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
)

type SingleTestImage struct {
	board     DevBoard
	successRe *regexp.Regexp
}

func NewSingleTestImage(board DevBoard, successRe *regexp.Regexp) *SingleTestImage {
	return &SingleTestImage{board, successRe}
}

func (i *SingleTestImage) RunTest(ctx context.Context) error {
	if i.successRe == nil {
		return errors.New("Success regexp not set")
	}
	i.board.Reset(ctx)
	_, err := i.board.ReadSerialSubmatch(ctx, i.successRe)
	if err != nil {
		return errors.Wrap(err, "Success regexp not detected")
	}
	return nil
}

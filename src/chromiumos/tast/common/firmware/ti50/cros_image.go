// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// CrOSImage interacts with a board running ti50.
type CrOSImage struct {
	*CommandImage
}

// NewCrOSImage creates a new CrOSImage.
func NewCrOSImage(board DevBoard) *CrOSImage {
	return &CrOSImage{CommandImage: NewCommandImage(board, "\r", "\n> ")}
}

// WaitUntilBooted waits until the image is fully booted.
func (i *CrOSImage) WaitUntilBooted(ctx context.Context) error {
	return i.CommandImage.WaitUntilBooted(ctx, 2*time.Second)
}

// CrOSImageHelpOutput holds relevant data from the 'help' command.
type CrOSImageHelpOutput struct {
	Raw      string
	Commands []string
}

// Help issues 'help' and parses its output.
func (i *CrOSImage) Help(ctx context.Context) (CrOSImageHelpOutput, error) {
	out, err := i.Command(ctx, "help")
	if err != nil {
		return CrOSImageHelpOutput{}, errors.Wrap(err, "failed to execute help command")
	}
	re := regexp.MustCompile(`(?s)Known commands:\s*(.*)HELP LIST`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		return CrOSImageHelpOutput{}, errors.New("failed to parse help output")
	}
	cmds := make([]string, 0)
	for _, c := range strings.Split(m[1], " ") {
		c = strings.TrimSpace(c)
		if len(c) > 0 {
			cmds = append(cmds, c)
		}
	}
	return CrOSImageHelpOutput{out, cmds}, nil
}

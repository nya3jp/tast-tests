// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

type Ti50Image struct {
	*CommandImage
}

func NewTi50Image(board DevBoard) *Ti50Image {
	return &Ti50Image{CommandImage: NewCommandImage(board, "\r", "\r\n> ")}
}

func (i *Ti50Image) WaitUntilBooted(ctx context.Context) error {
	return i.CommandImage.WaitUntilBooted(ctx, 2 * time.Second)
}

type Ti50HelpOutput struct {
	Raw      string
	Commands []string
}

/*
Known commands:
  bid             ec_comm         help            sleepmask       version
  brdprop         ecrst           powerbtn        sysinfo         wp
  ccd             gpioget         rddkeepalive    sysrst
  ccdstate        gpioset         reboot          usb
HELP LIST = more info; HELP CMD = help on CMD.
*/
func (i *Ti50Image) Help(ctx context.Context) (Ti50HelpOutput, error) {
	out, err := i.Command(ctx, "help")
	if err != nil {
		return Ti50HelpOutput{}, errors.Wrap(err, "Executing help command")
	}
	re := regexp.MustCompile(`(?s)Known commands:\s*(.*)HELP LIST`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		return Ti50HelpOutput{}, errors.New("Unable to parse help output")
	}
	cmds := make([]string, 0)
	for _, c := range strings.Split(m[1], " ") {
		c = strings.TrimSpace(c)
		if len(c) > 0 {
			cmds = append(cmds, c)
		}
	}
	return Ti50HelpOutput{out, cmds}, nil
}

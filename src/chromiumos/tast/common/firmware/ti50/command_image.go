// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/testing"
)

// CommandImage displays a prompt and responds to cli commands.
type CommandImage struct {
	board           DevBoard
	promptCmd       string
	promptPattern   string
	promptPatternRe *regexp.Regexp
}

// NewCommandImage creates a new CommandImage. Typical examples of promptCmd and
// promptPattern are "\r" and "\n\r> " respectively.
func NewCommandImage(board DevBoard, promptCmd, promptPattern string) *CommandImage {
	return &CommandImage{board, promptCmd, promptPattern, regexp.MustCompile(promptPattern)}
}

// RawCommand sends a command to the image and waits for the regex to be matched
// before returning the captured groups.  It does not append the promptCmd as
// opposed to Command.
func (i *CommandImage) RawCommand(ctx context.Context, rawCmd string, re *regexp.Regexp) ([]string, error) {
	if err := i.board.WriteSerial(ctx, []byte(rawCmd)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	match, err := i.board.ReadSerialSubmatch(ctx, re)
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0)
	for _, m := range match {
		ret = append(ret, string(m))
	}
	return ret, nil
}

// Command sends the command to the image and returns the output from after the
// command up to the next prompt.
func (i *CommandImage) Command(ctx context.Context, cmd string) (string, error) {
	rawCmd := cmd + i.promptCmd
	matches, err := i.RawCommand(ctx, rawCmd, regexp.MustCompile("(?s)"+regexp.QuoteMeta(rawCmd)+"(.*)"+i.promptPattern))
	if err != nil {
		return "", err
	}
	return matches[1], nil
}

// WaitUntilBooted by checking that prompts are consistently displayed.
func (i *CommandImage) WaitUntilBooted(ctx context.Context, interval time.Duration) error {
	return testing.Poll(ctx, func(ctx1 context.Context) error {
		for j := 0; j < 3; j++ {
			if err := i.GetPrompt(ctx1); err != nil {
				return err
			}
			if err := ctx1.Err(); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Interval: interval})
}

// GetPrompt gets a fresh prompt from the image by  the prompt.
func (i *CommandImage) GetPrompt(ctx context.Context) error {
	if err := i.board.FlushSerial(ctx); err != nil {
		return err
	}
	_, err := i.Command(ctx, "")
	return err
}

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

// An image that displays a prompt and responds to cli commands.
type CommandImage struct {
	board           DevBoard
	promptCmd       string
	promptPattern   string
	promptPatternRe *regexp.Regexp
}

func NewCommandImage(board DevBoard, promptCmd string, promptPattern string) *CommandImage {
	return &CommandImage{board, promptCmd, promptPattern, regexp.MustCompile(promptPattern)}
}

// Issue command as-is and return matches from regex.
func (i *CommandImage) RawCommand(ctx context.Context, rawCmd string, re *regexp.Regexp) ([]string, error) {
	if err := i.board.WriteSerial(ctx, []byte(rawCmd)); err != nil {
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

// Issue command and return all text found before the next prompt.
func (i *CommandImage) Command(ctx context.Context, cmd string) (string, error) {
	matches, err := i.RawCommand(ctx, cmd+"\r", regexp.MustCompile("(?s)"+cmd+"\r"+"(.*)"+i.promptPattern))
	if err != nil {
		return "", err
	}
	return matches[1], nil
}

// Wait until booted by checking that prompts are consistently displayed.
func (i *CommandImage) WaitUntilBooted(ctx context.Context, interval time.Duration) error {
	return testing.Poll(ctx, func(ctx1 context.Context) error {
		for j := 0; j < 3; j++ {
			if err := i.GetPrompt(ctx1); err != nil {
				return err
			}
			/*if err := ctx1.Err(); err != nil {
				return err
			}*/
		}
		return nil
	}, &testing.PollOptions{Interval: interval})
}

// Get the prompt.
func (i *CommandImage) GetPrompt(ctx context.Context) error {
	testing.ContextLog(ctx, "GetPrompt FlushSerial")
	if err := i.board.FlushSerial(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "GetPrompt WriteSerial")
	if err := i.board.WriteSerial(ctx, []byte(i.promptCmd)); err != nil {
	        testing.ContextLog(ctx, "GetPrompt WriteSerial Failed: ", err)
		return err
	}
	testing.ContextLog(ctx, "GetPrompt ReadSerialSubmatch")
	_, err := i.board.ReadSerialSubmatch(ctx, i.promptPatternRe)
	return err
}

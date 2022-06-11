// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioDecoder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that Assistant Audio Decoder service starts on demand",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantWithStartAudioDecoderOnDemand",
		Timeout:      5 * time.Minute,
	})
}

type processNotFoundError struct{}

func (e *processNotFoundError) Error() string {
	return "Assistant Audio Decoder utility process not found"
}

type processFoundError struct{}

func (e *processFoundError) Error() string {
	return "Assistant Audio Decoder utility process found"
}

func findAudioDecoderUtilityProcess() (*process.Process, error) {
	procs, err := chromeproc.GetUtilityProcesses()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get utility processes")
	}

	for _, proc := range procs {
		cmdline, err := proc.Cmdline()
		if err != nil {
			return nil, errors.New("failed to get cmdline")
		}
		if strings.Contains(cmdline, " --utility-sub-type=chromeos.assistant.mojom.AssistantAudioDecoderFactory") {
			return proc, nil
		}
	}
	return nil, &processNotFoundError{}
}

func expectNoAudioDecoderUtilityProcess() error {
	_, err := findAudioDecoderUtilityProcess()
	if _, ok := err.(*processNotFoundError); ok {
		return nil
	} else if err == nil {
		return &processFoundError{}
	}
	return err
}

func expectAudioDecoderUtilityProcess() error {
	_, err := findAudioDecoderUtilityProcess()
	return err
}

func AudioDecoder(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := testing.Poll(ctx,
		func(context.Context) error {
			return expectNoAudioDecoderUtilityProcess()
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Expect no audio decoder process is running: ", err)
	}

	_, err = assistant.SendTextQuery(ctx, tconn, "Play News")
	if err != nil {
		// TODO: Play News query does not get marked as completed. Update Chrome side API as it's not recognized as an error.
		s.Log("Failed to send Play News query: ", err)
	}

	if err := testing.Poll(ctx,
		func(context.Context) error {
			return expectAudioDecoderUtilityProcess()
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Expect audio decoder process is running: ", err)
	}

	_, err = assistant.SendTextQuery(ctx, tconn, "Stop News")
	if err != nil {
		s.Fatal("Failed to send Stop News query: ", err)
	}

	if err := testing.Poll(ctx,
		func(context.Context) error {
			return expectNoAudioDecoderUtilityProcess()
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Expect no audio decoder process is running: ", err)
	}
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"

	"github.com/golang/protobuf/descriptor"
	"github.com/golang/protobuf/jsonpb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// DPSLSendMessage is a helper function that creates and runs a
// diagnostics_dpsl_test_requester command over vsh. It accepts the name of
// command, the proto message to send, and a proto message to hold the output.
// See https://crrev.com/c/1767044 for a description of the
// diagnostics_dpsl_test_requester.
func DPSLSendMessage(ctx context.Context, msgName string, in, out descriptor.Message) error {
	m := jsonpb.Marshaler{}
	body, err := m.MarshalToString(in)
	if err != nil {
		_, md := descriptor.ForMessage(in)
		return errors.Wrapf(err, "unable to marshal %v to String", md.GetName())
	}

	cmd := vm.CreateVSHCommand(ctx, WilcoVMCID, "diagnostics_dpsl_test_requester",
		"--message_name="+msgName, "--message_body="+body)

	msg, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "unable to run diagnostics_dpsl_test_requester")
	}

	var r struct {
		Body json.RawMessage `json:"body"`
	}

	if err := json.Unmarshal(msg, &r); err != nil {
		return errors.Wrap(err, "unable to parse byte message to JSON")
	}

	if err := jsonpb.UnmarshalString(string(r.Body), out); err != nil {
		_, md := descriptor.ForMessage(out)
		return errors.Wrapf(err, "error parsing return msg to %v", md.GetName())
	}

	return nil
}

// DPSLReceiveMessage listens for events sent to the VM and parses them into the
// descriptor.Message. Once the command is running and listening for incming
// events, a signal will be sent through the channel. The function will block,
// trying to parse additional incoming messages until the correct message type
// is found or the context timeout is reached. Messages that do not match the
// provided descriptor.Message will be ignored.
func DPSLReceiveMessage(ctx context.Context, ch chan struct{}, out descriptor.Message) error {
	cmd := vm.CreateVSHCommand(ctx, WilcoVMCID, "diagnostics_dpsl_test_listener")
	buf, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "unable to get stdout for dpsl receive command")
	}
	dec := json.NewDecoder(buf)

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "unable to run diagnostics_dpsl_test_listener")
	}

	// Alert the client the utility is now listening.
	ch <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "timed out waiting for message")
		default:
			if dec.More() {
				var r struct {
					Body json.RawMessage `json:"body"`
				}
				// Check that the JSON object can be parsed, if not continue
				// parsing stdout.
				if err := dec.Decode(&r); err != nil {
					testing.ContextLogf(ctx, "Unable to decode JSON: %s. Continuing", err)
					continue
				}

				// Check that the Message can be parsed into the provided proto
				// message, if not log the message and continue parsing stdout.
				if err := jsonpb.UnmarshalString(string(r.Body), out); err != nil {
					_, md := descriptor.ForMessage(out)
					testing.ContextLogf(ctx, "Unable to unmarshal proto message to %s: %s. Continuing", md.GetName(), err)
					continue
				}

				return nil
			}
		}
	}
}

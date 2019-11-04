// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"syscall"

	"github.com/golang/protobuf/descriptor"
	"github.com/golang/protobuf/jsonpb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// DPSLMsg is the message format used by the sending and listening DPSL
// utilities.
type DPSLMsg struct {
	Name string
	Body json.RawMessage `json:"body"`
}

// DPSLMsgResult is a stuct that wraps a DPSLMsg and an error value. If the
// error is populated, the DPSLMsg is not valid.
type DPSLMsgResult struct {
	msg DPSLMsg
	err error
}

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

	var r DPSLMsg

	if err := json.Unmarshal(msg, &r); err != nil {
		return errors.Wrap(err, "unable to parse byte message to JSON")
	}

	if err := jsonpb.UnmarshalString(string(r.Body), out); err != nil {
		_, md := descriptor.ForMessage(out)
		return errors.Wrapf(err, "error parsing return msg to %v", md.GetName())
	}

	return nil
}

// DPSLMessageReceiver contains the necessary components to run the DPSL
// listening utility and parse its output.
type DPSLMessageReceiver struct {
	Msgs chan DPSLMsgResult
	stop chan struct{}
	cmd  *testexec.Cmd
	dec  *json.Decoder
}

// CreateDPSLMessageReceiver will start a utility inside of the Wilco VM
// listening for DPSL messages. It will return a DPSLMessageReceiver struct that
// decodes and buffers the JSON. It will immediately start consuming messages
// from the stdout of the dpsl test listener.
func CreateDPSLMessageReceiver(ctx context.Context) (*DPSLMessageReceiver, error) {
	rec := DPSLMessageReceiver{}
	rec.cmd = vm.CreateVSHCommand(ctx, WilcoVMCID, "diagnostics_dpsl_test_listener")
	buf, err := rec.cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get stdout for dpsl receive command")
	}
	rec.dec = json.NewDecoder(buf)

	if err := rec.cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "unable to run diagnostics_dpsl_test_listener")
	}

	rec.Msgs = make(chan DPSLMsgResult)
	rec.stop = make(chan struct{})

	go func() {
		for {
			select {
			case <-rec.stop:
				return
			case <-ctx.Done():
				rec.Msgs <- DPSLMsgResult{DPSLMsg{}, errors.Wrap(ctx.Err(), "timed out waiting for message")}
				return
			default:
				if rec.dec.More() {
					var m DPSLMsg
					// Check that the JSON object can be parsed, if not continue
					// parsing stdout.
					if err := rec.dec.Decode(&m); err != nil {
						testing.ContextLogf(ctx, "Unable to decode JSON: %s. Continuing", err)
						continue
					}

					rec.Msgs <- DPSLMsgResult{m, nil}
				}
			}
		}
	}()

	return &rec, nil
}

// Stop will stop the DPSLMessageReceiver gracefully by interrupting the DPSL
// listening program and exiting the goroutine.
func (rec *DPSLMessageReceiver) Stop() {
	close(rec.stop)
	// Clear the channel so the goroutine can exit if it is blocked on adding a
	// new message to the channel.
	if len(rec.Msgs) > 0 {
		<-rec.Msgs
	}
	rec.cmd.Signal(syscall.SIGINT)
	rec.cmd.Wait()
}

// WaitForMessage listens for events sent to the VM and attempt to parse them
// into the descriptor.Message. The function will block until the correct
// message type is found or the context timeout is reached. Messages that do not
// match the provided descriptor.Message will be ignored and discarded.
func (rec *DPSLMessageReceiver) WaitForMessage(ctx context.Context, out descriptor.Message) error {
	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "time out waiting for proto message")
		case r := <-rec.Msgs:
			if r.err != nil {
				return errors.Wrap(r.err, "unable to wait for proto message")
			}
			// Check that the Message can be parsed into the provided proto
			// message, if not log the message and continue parsing stdout.
			if err := jsonpb.UnmarshalString(string(r.msg.Body), out); err != nil {
				_, md := descriptor.ForMessage(out)
				testing.ContextLogf(ctx, "Unable to unmarshal proto message to %s: %s. Continuing",
					md.GetName(), err)
				continue
			}

			return nil
		}
	}
}

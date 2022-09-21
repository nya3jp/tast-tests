// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"bufio"
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/descriptor"
	"github.com/golang/protobuf/jsonpb"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

// dpslMsg is the message format used by the sending and listening DPSL
// utilities.
type dpslMsg struct {
	Name string
	Body json.RawMessage `json:"body"`
}

// dpslMsgResult is a stuct that wraps a dpslMsg and an error value. If the
// error is populated, the dpslMsg is not valid.
type dpslMsgResult struct {
	msg dpslMsg
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
		return errors.Wrapf(err, "unable to marshal %v to string", md.GetName())
	}

	cmd := vm.CreateVSHCommand(ctx, WilcoVMCID, "diagnostics_dpsl_test_requester",
		"--message_name="+msgName, "--message_body="+body)

	msg, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "unable to run diagnostics_dpsl_test_requester")
	}

	var r dpslMsg

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
	msgs     chan dpslMsgResult
	stop     chan struct{}
	cmd      *testexec.Cmd
	dec      *json.Decoder
	response *dtcpb.HandleMessageFromUiResponse
}

type option func(*DPSLMessageReceiver)

// WithHandleMessageFromUiResponse sets the --ui_response_body flag, making
// the listener reply with that value to HandleMessageFromUi requests.
func WithHandleMessageFromUiResponse(response *dtcpb.HandleMessageFromUiResponse) option { // NOLINT
	return func(rec *DPSLMessageReceiver) {
		rec.response = response
	}
}

// NewDPSLMessageReceiver will start a utility inside of the Wilco VM
// listening for DPSL messages. It will return a DPSLMessageReceiver struct that
// decodes and buffers the JSON. It will immediately start consuming messages
// from the stdout of the dpsl test listener.
func NewDPSLMessageReceiver(ctx context.Context, opts ...option) (*DPSLMessageReceiver, error) {
	rec := DPSLMessageReceiver{}

	for _, opt := range opts {
		opt(&rec)
	}

	if rec.response != nil {
		m := jsonpb.Marshaler{}
		body, err := m.MarshalToString(rec.response)
		if err != nil {
			_, md := descriptor.ForMessage(rec.response)
			return nil, errors.Wrapf(err, "unable to marshal %v to string", md.GetName())
		}
		rec.cmd = vm.CreateVSHCommand(ctx, WilcoVMCID, "diagnostics_dpsl_test_listener", "--ui_response_body="+body)
	} else {
		rec.cmd = vm.CreateVSHCommand(ctx, WilcoVMCID, "diagnostics_dpsl_test_listener")
	}

	buferr, err := rec.cmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get stderr for the dpsl receiver")
	}
	scanerr := bufio.NewScanner(buferr)
	go func() {
		for scanerr.Scan() {
			testing.ContextLogf(ctx, "dpsl receiver error: %s", scanerr.Text())
		}

		if err := scanerr.Err(); err != nil {
			testing.ContextLog(ctx, "Failed to read stderr from the dpsl receiver due to: ", err)
		}
	}()
	buf, err := rec.cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get stdout for the dpsl receiver")
	}
	rec.dec = json.NewDecoder(buf)

	if err := rec.cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "unable to run diagnostics_dpsl_test_listener")
	}

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := waitVMGRPCServerReady(readyCtx, wilcoVMUIMessageReceiverDTCPort); err != nil {
		return nil, errors.Wrap(err, "diagnostics_dpsl_test_listener did not become ready")
	}

	// rec.msgs has a buffer size of 2 to prevent blocking on sending a single
	// message. This prevents a race condition that could potentially leak the
	// goroutine if the receiver was stopped before the message is sent.
	rec.msgs = make(chan dpslMsgResult, 2)
	rec.stop = make(chan struct{})

	go func() {
		for {
			select {
			case <-rec.stop:
				return
			case <-ctx.Done():
				rec.msgs <- dpslMsgResult{dpslMsg{}, errors.Wrap(ctx.Err(), "timed out waiting for message")}
				return
			default:
				// dec.More() blocks until data has been received. It will
				// return false when the command has been stopped.
				if rec.dec.More() {
					var m dpslMsg
					if err := rec.dec.Decode(&m); err != nil {
						rec.msgs <- dpslMsgResult{m, errors.Wrap(err, "unable to decode JSON")}
						return
					}

					rec.msgs <- dpslMsgResult{m, nil}
				} else {
					return
				}
			}
		}
	}()

	return &rec, nil
}

// Stop will stop the DPSLMessageReceiver gracefully by interrupting the DPSL
// listening program and exiting the goroutine.
func (rec *DPSLMessageReceiver) Stop(ctx context.Context) {
	close(rec.stop)

	if err := rec.cmd.Signal(unix.SIGINT); err != nil {
		testing.ContextLog(ctx, "Failed to send signal to the dpsl receiver: ", err)
	}

	// Clear the channel so the goroutine can exit if it is blocked on adding a
	// new message to the channel.
	for len(rec.msgs) > 0 {
		<-rec.msgs
	}
}

// WaitForMessage listens for events sent to the VM and attempt to parse them
// into the descriptor.Message. The function will block until the correct
// message type is found or the context timeout is reached. Messages that do not
// match the provided descriptor.Message will be ignored and discarded.
func (rec *DPSLMessageReceiver) WaitForMessage(ctx context.Context, out descriptor.Message) error {
	_, md := descriptor.ForMessage(out)
	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "time out waiting for proto message")
		case m := <-rec.msgs:
			if m.err != nil {
				return errors.Wrap(m.err, "unable to wait for proto message")
			}

			if md.GetName() != m.msg.Name {
				testing.ContextLogf(ctx, "Received proto message %v, but waiting for %v. Continuing",
					m.msg.Name, md.GetName())
				continue
			}
			// Check that the Message can be parsed into the provided proto
			// message, if not log the message and continue parsing stdout.
			if err := jsonpb.UnmarshalString(string(m.msg.Body), out); err != nil {
				testing.ContextLogf(ctx, "Unable to unmarshal proto message to %s: %s. Continuing",
					md.GetName(), err)
				continue
			}

			return nil
		}
	}
}

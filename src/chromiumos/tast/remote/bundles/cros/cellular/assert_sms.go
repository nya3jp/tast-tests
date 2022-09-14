// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssertSMS,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that an SMS message sent from the callbox is received",
		Contacts: []string{
			"jstanko@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_callbox"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.cellular.RemoteCellularService"},
		Fixture:      "callboxManagedFixture",
		Timeout:      5 * time.Minute,
	})
}

func AssertSMS(ctx context.Context, s *testing.State) {
	dutConn := s.DUT().Conn()
	tf := s.FixtValue().(*manager.TestFixture)
	if err := tf.ConnectToCallbox(ctx, dutConn, &manager.ConfigureCallboxRequestBody{
		Hardware:     "CMW",
		CellularType: "LTE",
		ParameterList: []string{
			"band", "2",
			"bw", "20",
			"mimo", "2x2",
			"tm", "1",
			"pul", "0",
			"pdl", "high",
		},
	}); err != nil {
		s.Fatal("Failed to initialize cellular connection: ", err)
	}

	// start goroutine to watch for an SMS message on the DUT
	errorCh := make(chan error, 1)
	messageCh := make(chan string, 1)
	go func() {
		defer func() {
			close(errorCh)
			close(messageCh)
		}()
		if resp, err := tf.RemoteCellularClient.WaitForNextSms(ctx, &empty.Empty{}); err != nil {
			errorCh <- err
		} else {
			messageCh <- resp.Message.Text
		}
	}()

	// Send a message and poll until it is received on the DUT
	attempts := 0
	testMessage := "Hello " + time.Now().Format(time.UnixDate)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		attempts++
		testing.ContextLogf(ctx, "Sending SMS, attempt %d", attempts)
		if err := tf.CallboxManagerClient.SendSms(ctx, &manager.SendSmsRequestBody{Message: testMessage}); err != nil {
			s.Fatal("Failed to send SMS from callbox: ", err)
		}

		// poll for WaitForNextSMS to exit
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if len(messageCh) != 0 || len(errorCh) != 0 {
				return nil
			}
			return errors.Errorf("failed to wait for DUT to receive SMS %q", testMessage)
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return err
		}

		return nil
	}, &testing.PollOptions{}); err != nil {
		s.Fatal("Failed to wait for SMS on DUT: ", <-errorCh)
	}

	if len(errorCh) > 0 {
		s.Fatal("Failed to receive SMS message on DUT: ", <-errorCh)
	}

	receivedMessage := <-messageCh
	if testMessage != receivedMessage {
		s.Fatalf("Failed to check SMS message, got %q, expected %q", receivedMessage, testMessage)
	}
}

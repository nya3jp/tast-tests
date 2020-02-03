// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConsentSanity,
		Desc: "Make sure Tast tests can enable consents reliably",
		Contacts: []string{
			"mutexlox@chromium.org",
			"iby@chromium.org",
			"cros-monitoring-forensics@google.com",
			"nya@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
	})
}

func ConsentSanity(ctx context.Context, s *testing.State) {
	const consentPath = "/home/chronos/Consent To Send Stats"

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := crash.SetConsent(ctx, cr, true); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	b, err := ioutil.ReadFile(consentPath)
	if err != nil {
		s.Fatal("Failed to read consent file: ", err)
	}

	initUUID := string(b)
	s.Logf("Initial consent UUID is %q", initUUID)

	uuids := make(map[string]struct{})
	uuids[initUUID] = struct{}{}
	var firstErr error

	// Poll the consent file for 10 seconds.
	for i := 0; i < 100; i++ {
		if err := testing.Sleep(ctx, 100*time.Millisecond); err != nil {
			s.Fatal("Failed during sleep: ", err)
		}
		b, err := ioutil.ReadFile(consentPath)
		if err != nil {
			s.Log("Failed to read consent file: ", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		uuid := string(b)
		s.Logf("Current consent UUID is %q", uuid)
		uuids[uuid] = struct{}{}
	}

	if firstErr != nil {
		s.Error("Failed to read consent file: ", firstErr)
	}

	if len(uuids) >= 2 {
		s.Error("Multiple consent UUIDs were observed")
	}
}

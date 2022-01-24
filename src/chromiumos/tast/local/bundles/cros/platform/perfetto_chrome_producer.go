// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoChromeProducer,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests Chrome connecting to the Perfetto system tracing service",
		Contacts:     []string{"chinglinyu@chromium.org", "chenghaoyang@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Attr:         []string{"group:mainline"},
	})
}

// waitForChromeProducer waits until the required number of Chrome producers are connected to the system tracing service daemon.
func waitForChromeProducer(ctx context.Context) error {
	// At least 2 Chrome producers (browser, renderer, utility, etc).
	const requiredChromeProducers = 2

	return testing.Poll(ctx, func(context.Context) error {
		// "perfetto --query-raw" outputs the TracingServiceState proto message.
		cmd := testexec.CommandContext(ctx, "/usr/bin/perfetto", "--query-raw")

		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to query the service state of traced")
		}

		st := perfetto_proto.TracingServiceState{}
		// Count the number of producers named "org.chromium-(process ID)".
		chromeProducers := 0
		if err = proto.Unmarshal(out, &st); err != nil {
			return errors.Wrap(err, "failed to parse the service state output of traced")
		}

		// Example chrome producer (in pbtxt):
		// producers: {
		//   id: 192
		//   name: "org.chromium-31550"
		//   uid: 1000
		//   sdk_version: "Perfetto v0.0 (unknown)"
		// }
		re := regexp.MustCompile(`^org.chromium-\d+$`)
		for _, prd := range st.GetProducers() {
			if re.MatchString(prd.GetName()) {
				chromeProducers++
			}
		}

		if chromeProducers < requiredChromeProducers {
			return errors.Errorf("unexpected number (%d) of Chrome producer connected", chromeProducers)
		}

		return nil
	}, &testing.PollOptions{
		// Chrome producers retry the connection with delay on failure to connect. Poll using a 30 second timeout.
		Timeout: 30 * time.Second,
	})
}

// PerfettoChromeProducer tests Chrome as a perfetto trace producer.
// The test enables the "EnablePerfettoSystemTracing" feature flag for Chrome and then checks if traced sees multiple Chrome producers connected.
func PerfettoChromeProducer(ctx context.Context, s *testing.State) {
	_, _, err := tracing.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running: ", err)
	}

	if err = waitForChromeProducer(ctx); err != nil {
		s.Fatal("Failed in waiting for Chrome producers: ", err)
	}
}

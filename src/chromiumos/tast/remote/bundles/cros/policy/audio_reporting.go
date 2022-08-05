// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/reportingutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

const audioReportingEnabledUser = "policy.AudioReporting.enabled_username"
const audioReportingEnabledPassword = "policy.AudioReporting.password"

type audioReportingParameters struct {
	usernamePath     string // username for Chrome enrollment
	passwordPath     string // password for Chrome enrollment
	reportingEnabled bool   // test should expect reporting enabled
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioReporting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify reporting of audio telemetry for enterprise devices",
		Contacts: []string{
			"albertojuarez@google.com",       // Test owner
			"cros-reporting-team@google.com", // Team mailing list
		},
		Attr:         []string{"group:dpanel-end2end", "group:enterprise-reporting"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.hwsec.OwnershipService", "tast.cros.tape.Service"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "audio_reporting_enabled",
				Val: audioReportingParameters{
					usernamePath:     audioReportingEnabledUser,
					passwordPath:     audioReportingEnabledPassword,
					reportingEnabled: true,
				},
			}, {
				Name: "audio_reporting_disabled",
				Val: audioReportingParameters{
					usernamePath:     reportingutil.ReportingPoliciesDisabledUser,
					passwordPath:     reportingutil.ReportingPoliciesDisabledPassword,
					reportingEnabled: false,
				},
			},
		},
		VarDeps: []string{
			audioReportingEnabledUser,
			audioReportingEnabledPassword,
			reportingutil.ReportingPoliciesDisabledUser,
			reportingutil.ReportingPoliciesDisabledPassword,
			reportingutil.ManagedChromeCustomerIDPath,
			reportingutil.EventsAPIKeyPath,
			tape.ServiceAccountVar,
		},
	})
}

func audioTelemetry(event reportingutil.InputEvent) *reportingutil.AudioTelemetry {
	if w := event.WrappedEncryptedData; w != nil {
		if m := w.MetricData; m != nil {
			if i := m.TelemetryData; i != nil {
				if m := i.AudioTelemetry; m != nil {
					return m
				}
			}
		}
	}
	return nil
}

func validateAudioTelemetry(ctx context.Context, audioTelemetry reportingutil.AudioTelemetry, deviceName string) error {
	if audioTelemetry.OutputMute == true {
		return errors.Errorf("failed to verify OutputMute: %v", audioTelemetry.OutputMute)
	}
	if audioTelemetry.InputMute == true {
		return errors.Errorf("failed to verify InputMute: %v", audioTelemetry.InputMute)
	}
	if audioTelemetry.OutputVolume != 10 {
		return errors.Errorf("failed to verify OutputVolume: %v", audioTelemetry.OutputVolume)
	}
	if audioTelemetry.OutputDeviceName != deviceName {
		return errors.Errorf("failed to verify OutputDeviceName: %v", audioTelemetry.OutputDeviceName)
	}
	if audioTelemetry.InputGain <= 0 {
		return errors.Errorf("failed to verify InputGain: %v", audioTelemetry.InputGain)
	}
	if audioTelemetry.InputDeviceName != "" {
		return errors.Errorf("failed to verify InputDeviceName: %v", audioTelemetry.InputDeviceName)
	}
	return nil
}

func AudioReporting(ctx context.Context, s *testing.State) {
	param := s.Param().(audioReportingParameters)
	user := s.RequiredVar(param.usernamePath)
	pass := s.RequiredVar(param.passwordPath)
	cID := s.RequiredVar(reportingutil.ManagedChromeCustomerIDPath)
	APIKey := s.RequiredVar(reportingutil.EventsAPIKeyPath)
	sa := []byte(s.RequiredVar(tape.ServiceAccountVar))

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	defer reportingutil.Deprovision(ctx, cl.Conn, sa, cID)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	testStartTime := time.Now()
	if _, err := pc.GAIAEnrollForReporting(ctx, &ps.GAIAEnrollForReportingRequest{
		Username:           user,
		Password:           pass,
		DmserverUrl:        reportingutil.DmServerURL,
		ReportingServerUrl: reportingutil.ReportingServerURL,
		EnabledFeatures:    "EncryptedReportingPipeline, EnableTelemetryTestingRates",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}

	c, err := pc.ClientID(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to grab client ID from device: ", err)
	}
	/*
		// Give 5 seconds to cleanup other resources.
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Stop UI in advance for this test to avoid the node being selected by UI.
		if err := upstart.StopJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to stop ui: ", err)
		}
		defer upstart.EnsureJobRunning(ctx, "ui")

		// Setting the volume to low level.
		cras, err := audio.NewCras(ctx)
		if err != nil {
			s.Fatal("Failed to create Cras object: ", err)
		}

		vh, err := audio.NewVolumeHelper(ctx)
		if err != nil {
			s.Fatal("Failed to create the volumeHelper: ", err)
		}
		originalVolume, err := vh.GetVolume(ctx)
		if err != nil {
			s.Fatal("Failed to get volume: ", err)
		}
		testVol := 10
		s.Logf("Setting Output node volume to %d", testVol)
		if err := vh.SetVolume(ctx, testVol); err != nil {
			s.Errorf("Failed to set output node volume to %d: %v", testVol, err)
		}
		defer vh.SetVolume(cleanupCtx, originalVolume)

		expectedAudioNode := "INTERNAL_SPEAKER"
		// Setting the active node to INTERNAL_SPEAKER if default node is set to some other node.
		if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
			s.Fatalf("Failed to select active device %q: %v", expectedAudioNode, err)
		}
		deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device: ", err)
		}
		if deviceType != expectedAudioNode {
			s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
		}

		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			s.Fatal("Failed to detect running output device: ", err)
		}

		if deviceName != devName {
			s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
	*/
	/*
		// Stop UI in advance for this test to avoid the node being selected by UI.
		if err := upstart.StopJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to stop ui: ", err)
		}
		defer upstart.EnsureJobRunning(ctx, "ui")

		// Use a shorter context to save time for cleanup.
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		cras, err := audio.NewCras(ctx)
		if err != nil {
			s.Fatal("Failed to connect to CRAS: ", err)
		}

		// Only test on internal mic and internal speaker until below demands are met.
		// 1. Support label to force the test run on DUT having a headphone jack. (crbug.com/936807)
		// 2. Have a method to get correct PCM name from CRAS. (b/142910355).
		if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
			s.Fatal("Failed to set internal mic active: ", err)
		}

		if err := cras.SetActiveNodeByType(ctx, "INTERNAL_SPEAKER"); err != nil {
			s.Fatal("Failed to set internal speaker active: ", err)
		}

		//crasNodes, err := cras.GetNodes(ctx)
		//if err != nil {
		//	s.Fatal("Failed to obtain CRAS nodes: ", err)
		//}

		// Stop CRAS to make sure the audio device won't be occupied.
		s.Log("Stopping CRAS")
		if err := upstart.StopJob(ctx, "cras"); err != nil {
			s.Fatal("Failed to stop CRAS: ", err)
		}
	*/
	// Events sent from the metric reporting manager won't be reported for the first minute.
	if err = testing.Sleep(ctx, 4*time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err := reportingutil.LookupEvents(ctx, reportingutil.ReportingServerURL, cID, APIKey, "TELEMETRY_METRIC")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to look up events"))
		}

		prunedEvents, err := reportingutil.PruneEvents(ctx, events, c.ClientId, testStartTime, func(e reportingutil.InputEvent) bool {
			return audioTelemetry(e) != nil
		})
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to prune events"))
		}

		if !param.reportingEnabled && len(prunedEvents) == 0 {
			return nil
		}
		if !param.reportingEnabled && len(prunedEvents) > 0 {
			return testing.PollBreak(errors.New("events found when reporting is disabled"))
		}
		if param.reportingEnabled && len(prunedEvents) > 1 {
			return testing.PollBreak(errors.New("more than one event reporting"))
		}
		if param.reportingEnabled && len(prunedEvents) == 0 {
			return errors.New("no events found while reporting enabled")
		}
		if err = validateAudioTelemetry(ctx, *audioTelemetry(prunedEvents[0]), "INTERNAL_SPEAKER"); err != nil {
			return testing.PollBreak(errors.Wrap(err, "invalid event"))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  1 * time.Minute,
		Interval: 30 * time.Second,
	}); err != nil {
		s.Errorf("Failed to validate audio telemetry: %v:", err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// Prepare prepares conference room link before testing.
type Prepare func() (string, error)

// Cleanup releases the resources which the case used.
type Cleanup func()

// Conference contains user's operation when enter a confernece room.
type Conference interface {
	Join(context.Context, string) error
	AdmitParticipant(context.Context) error
	VideoAudioControl(context.Context) error
	SwitchTabs(context.Context) error
	ChangeLayout(context.Context) error
	BackgroundBlurring(context.Context) error
	ExtendedDisplayPresenting(context.Context, bool) error
	PresentSlide(context.Context) error
	StopPresenting(context.Context) error
	End(context.Context) error
}

// MeetConference runs the specified user scenario in conference room with different CUJ performance level.
func MeetConference(ctx context.Context, cr *chrome.Chrome, conf Conference, prepare Prepare, cleanup Cleanup, tier, tmpPath string, tabletMode, extendedDisplay bool) error {
	inviteLink := ""
	if prepare != nil {
		url, err := prepare()
		if err != nil {
			return err
		}
		inviteLink = url
	}
	if cleanup != nil {
		defer cleanup()
	}
	if inviteLink == "" {
		return errors.New("failed to get inviteLink")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

	testing.ContextLog(ctx, "Start to get browser start time")
	//browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	//if err != nil {
	//	return errors.Wrap(err, "failed to get browser start time")
	//}

	if err := conf.Join(ctx, inviteLink); err != nil {
		return err
	}

	defer func() {
		conf.End(ctx)
	}()

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(ctx)

	testing.ContextLog(ctx, "Start recording actions")

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(ctx)

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		errc := make(chan error)
		go func() {
			errc <- graphics.MeasureGPUCounters(ctx, 10*time.Second, pv)
		}()

		// Basic
		if err := conf.SwitchTabs(ctx); err != nil {
			return err
		}

		if err := conf.VideoAudioControl(ctx); err != nil {
			return err
		}

		if err := conf.ChangeLayout(ctx); err != nil {
			return err
		}

		// Plus and premium tier.
		if tier == "plus" || tier == "premium" {
			if extendedDisplay {
				if err := conf.ExtendedDisplayPresenting(ctx, tabletMode); err != nil {
					return err
				}
			} else {
				if err := conf.PresentSlide(ctx); err != nil {
					return err
				}
				if err := conf.StopPresenting(ctx); err != nil {
					return err
				}
			}
		}

		// Premium tier.
		if tier == "premium" {
			if err := conf.BackgroundBlurring(ctx); err != nil {
				return err
			}
		}

		if err := <-errc; err != nil {
			return errors.Wrap(err, "failed to collect GPU counters")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to conduct the recorder task")
	}

	if err := recorder.Record(ctx, pv); err != nil {
		return errors.Wrap(err, "failed to record the data")
	}

	// pv.Set(perf.Metric{
	// Name:      "Browser.StartTime",
	// Unit:      "ms",
	// Direction: perf.SmallerIsBetter,
	// }, float64(browserStartTime.Milliseconds()))

	if err := pv.Save(tmpPath); err != nil {
		return errors.Wrap(err, "failed to save perf data")
	}

	if err := recorder.SaveHistograms(tmpPath); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}

	return nil
}

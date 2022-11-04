// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/services/cros/cellular"
	"chromiumos/tast/testing"
)

const (
	calibrationSampleSize = 100
	calibrationTestCount  = 5
)

// TxResult represents a Tx power measurement result.
type TxResult struct {
	Min               float64
	Max               float64
	Average           float64
	StandardDeviation float64
}

// TxMeasurementConfiguration represents the configuration of a Tx power measurement session.
type TxMeasurementConfiguration struct {
	TestPower        float64
	CalibrationPower float64
	SarLevel         int
	SampleCount      int
}

// TxMeasurementSession represents a Tx power measurement session consisting of multiple runs.
type TxMeasurementSession struct {
	callboxClient *manager.CallboxManagerClient
	dutClient     cellular.RemoteCellularServiceClient
}

// NewTxMeasurementSession creates a new TxMeasurementSession.
func NewTxMeasurementSession(callboxClient *manager.CallboxManagerClient, dutClient cellular.RemoteCellularServiceClient) *TxMeasurementSession {
	return &TxMeasurementSession{
		callboxClient: callboxClient,
		dutClient:     dutClient,
	}
}

// Run completes a Tx power measurement with the provided configuration.
func (t *TxMeasurementSession) Run(ctx context.Context, config *TxMeasurementConfiguration) (*TxResult, error) {
	if _, err := t.dutClient.DisableSar(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to disable SAR on DUT")
	}

	calibrationOffset, err := t.calibrate(ctx, config.CalibrationPower)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calibrate uplink power")
	}
	testing.ContextLogf(ctx, "Using calibration offset: %f", calibrationOffset)

	if _, err := t.dutClient.EnableSar(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to disable SAR on DUT")
	}
	if _, err := t.dutClient.ConfigureSar(ctx, &cellular.ConfigureSarRequest{PowerLevel: int64(config.SarLevel)}); err != nil {
		return nil, errors.Wrap(err, "failed to configure SAR on DUT")
	}

	if err := t.setUplinkPower(ctx, config.TestPower); err != nil {
		return nil, errors.Wrap(err, "failed to set uplink power on callbox")
	}

	result, err := t.runOnce(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run Tx measurement session")
	}

	// shift results to include calibration (standard deviation is not impacted by shifting mean)
	result.Average += calibrationOffset
	result.Min += calibrationOffset
	result.Max += calibrationOffset
	return result, nil
}

// Stop terminates any Tx measurement sessions running on the callbox.
func (t *TxMeasurementSession) Stop(ctx context.Context) error {
	if err := t.callboxClient.StopTxMeasurement(ctx, &manager.StopTxMeasurementRequestBody{}); err != nil {
		return errors.Wrap(err, "failed to stop Tx measurement session on callbox")
	}
	return nil
}

// Close terminates any Tx measurement sessions running on the callbox and releases any resources held open.
func (t *TxMeasurementSession) Close(ctx context.Context) error {
	if err := t.callboxClient.CloseTxMeasurement(ctx, &manager.CloseTxMeasurementRequestBody{}); err != nil {
		return errors.Wrap(err, "failed to stop Tx measurement session on callbox")
	}
	return nil
}

// calibrate averages multiple tx measurements on the callbox to make sure it's prepared for use and returns the measurement offset between the requested and received power (in dBm).
func (t *TxMeasurementSession) calibrate(ctx context.Context, power float64) (float64, error) {
	if err := t.setUplinkPower(ctx, power); err != nil {
		return 0, err
	}

	// measure the Tx power multiple times at this power level
	// Note: we don't have to run the test multiple times since the callbox will already average the results over multiple samples,
	// but it may be helpful to log multiple tests to spot any large variations from stopping/starting tests before running the main test.
	config := &TxMeasurementConfiguration{SampleCount: calibrationSampleSize}
	calibrationAvg := 0.0
	for j := 0; j < calibrationTestCount; j++ {
		result, err := t.runOnce(ctx, config)
		if err != nil {
			return 0, errors.Wrap(err, "failed to run Tx measurement session")
		}

		testing.ContextLogf(ctx, "Requested power: %f, received power: %f, offset: %f", power, result.Average, power-result.Average)
		calibrationAvg += power - result.Average
	}

	return calibrationAvg / float64(calibrationTestCount), nil
}

// runOnce runs a single Tx measurement session.
func (t *TxMeasurementSession) runOnce(ctx context.Context, config *TxMeasurementConfiguration) (*TxResult, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer t.Stop(cleanupCtx)

	if err := t.callboxClient.ConfigureTxMeasurement(ctx, &manager.ConfigureTxMeasurementRequestBody{SampleCount: config.SampleCount}); err != nil {
		return nil, errors.Wrap(err, "failed to configure Tx measurement")
	}
	if err := t.callboxClient.RunTxMeasurement(ctx, &manager.RunTxMeasurementRequestBody{}); err != nil {
		return nil, errors.Wrap(err, "failed to run Tx measurement on the callbox")
	}
	resp, err := t.callboxClient.FetchTxMeasurement(ctx, &manager.FetchTxMeasurementRequestBody{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Tx measurement results")
	}

	return &TxResult{
		Min:               resp.Min,
		Max:               resp.Max,
		Average:           resp.Average,
		StandardDeviation: resp.StandardDeviation,
	}, nil
}

func (t *TxMeasurementSession) setUplinkPower(ctx context.Context, power float64) error {
	req := &manager.ConfigureTxPowerRequestBody{Power: manager.NewTxPower(power)}
	if err := t.callboxClient.ConfigureTxPower(ctx, req); err != nil {
		return errors.Wrap(err, "failed to set Tx power on callbox")
	}
	return nil
}

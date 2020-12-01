/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.text.DecimalFormat;
import java.util.concurrent.TimeUnit;

import android.content.Context;
import android.util.Log;
import android.view.WindowManager;

/**
 * Base class for a test that performs bisection to determine maximum performance of a metric test
 * measures.
 */
public abstract class BaseTest {
    private static final String TAG = "BaseTest";

    // Time to wait for render warm up. No statistics is collected during this pass.
    private static final long WARM_UP_TIME = TimeUnit.SECONDS.toMillis(5);

    // Perform pass to probe the configuration using iterations. After each iteration current FPS is
    // checked and if it looks obviously bad, pass gets stopped earlier. Once all iterations are
    // done and final FPS is above PASS_THRESHOLD pass to probe is considered successful.
    private static final long TEST_ITERATION_TIME = TimeUnit.SECONDS.toMillis(8);
    private static final int TEST_ITERATION_COUNT = 3;

    // FPS pass test threshold, in ratio from ideal FPS, that matches device
    // refresh rate.
    private static final double PASS_THRESHOLD = 0.95;
    // FPS threshold, in ratio from ideal FPS, to identify that current pass to probe is obviously
    // bad and to stop pass earlier.
    private static final double OBVIOUS_BAD_THRESHOLD = 0.90;

    private static DecimalFormat DOUBLE_FORMATTER = new DecimalFormat("#.##");

    private final GamePerformanceActivity mActivity;

    // Device's refresh rate.
    private final double mRefreshRate;

    public BaseTest(GamePerformanceActivity activity) {
        mActivity = activity;
        final WindowManager windowManager =
                (WindowManager) getContext().getSystemService(Context.WINDOW_SERVICE);
        mRefreshRate = windowManager.getDefaultDisplay().getRefreshRate();
    }

    public Context getContext() {
        return mActivity;
    }

    public GamePerformanceActivity getActivity() {
        return mActivity;
    }

    // Returns name of the test.
    public abstract String getName();

    // Returns unit name.
    public abstract String getUnitName();

    // Returns number of measured units per one bisection unit.
    public abstract double getUnitScale();

    // Initializes test.
    public abstract void initUnits(double unitCount);

    // Initializes probe pass.
    protected abstract void initProbePass(int probe);

    // Frees probe pass.
    protected abstract void freeProbePass();

    /**
     * Performs the test and returns maximum number of measured units achieved. Unit is test
     * specific and name is returned by getUnitName. Returns 0 in case of failure.
     */
    public double run() {
        try {
            Log.i(TAG, "Test started " + getName());

            final double passFps = PASS_THRESHOLD * mRefreshRate;
            final double obviousBadFps = OBVIOUS_BAD_THRESHOLD * mRefreshRate;

            // Bisection bounds. Probe value is taken as middle point. Then it used to initialize
            // test with probe * getUnitScale units. In case probe passed, lowLimit is updated to
            // probe, otherwise upLimit is updated to probe. lowLimit contains probe that passes
            // and upLimit contains the probe that fails. Each iteration narrows the range.
            // Iterations continue until range is collapsed and lowLimit contains actual test
            // result.
            int lowLimit = 0; // Initially 0, that is recognized as failure.
            int upLimit = 250;

            while (true) {
                int probe = (lowLimit + upLimit) / 2;
                if (probe == lowLimit) {
                    Log.i(
                            TAG,
                            "Test done: "
                                    + DOUBLE_FORMATTER.format(probe * getUnitScale())
                                    + " "
                                    + getUnitName());
                    return probe * getUnitScale();
                }

                Log.i(
                        TAG,
                        "Start probe: "
                                + DOUBLE_FORMATTER.format(probe * getUnitScale())
                                + " "
                                + getUnitName());
                initProbePass(probe);

                Thread.sleep(WARM_UP_TIME);

                getActivity().resetFrameTimes();

                double fps = 0.0f;
                for (int i = 0; i < TEST_ITERATION_COUNT; ++i) {
                    Thread.sleep(TEST_ITERATION_TIME);
                    fps = getActivity().getFps();
                    if (fps < obviousBadFps) {
                        // Stop test earlier, we could not fit the loading.
                        break;
                    }
                }

                freeProbePass();

                Log.i(
                        TAG,
                        "Finish probe: "
                                + DOUBLE_FORMATTER.format(probe * getUnitScale())
                                + " "
                                + getUnitName()
                                + " - "
                                + DOUBLE_FORMATTER.format(fps)
                                + " FPS.");
                if (fps < passFps) {
                    upLimit = probe;
                } else {
                    lowLimit = probe;
                }
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return 0;
        }
    }
}

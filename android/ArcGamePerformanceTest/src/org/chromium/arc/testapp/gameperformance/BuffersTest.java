/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.io.File;
import java.io.IOException;
import java.util.Map;
import java.util.concurrent.CountDownLatch;

import android.app.Activity;
import android.os.Bundle;
import android.test.ActivityInstrumentationTestCase2;
import android.util.Log;

public class BuffersTest extends ActivityInstrumentationTestCase2<GamePerformanceActivity> {
    private static final String TAG = "BuffersTest";

    public BuffersTest() {
        super(GamePerformanceActivity.class);
    }

    public void testGraphicBufferMetrics() throws IOException, InterruptedException {
        final Bundle status = new Bundle();

        for (int i = 0; i < 2; ++i) {
            if (i == 0) {
                getActivity().attachSurfaceView();
            } else {
                getActivity().attachOpenGLView();
            }

            // Perform warm-up.
            Thread.sleep(2000);

            // Once atrace is done, this one is triggered.
            CountDownLatch latch = new CountDownLatch(1);

            final String passTag = i == 0 ? "surface" : "opengl";
            final String output =
                    (new File(
                                    getInstrumentation().getContext().getFilesDir(),
                                    "atrace_" + passTag + ".log"))
                            .getAbsolutePath();
            Log.i(TAG, "Collecting traces to " + output);
            new ATraceRunner(
                            getInstrumentation(),
                            output,
                            5,
                            "gfx",
                            new ATraceRunner.Delegate() {
                                @Override
                                public void onProcessed(boolean success) {
                                    latch.countDown();
                                }
                            })
                    .execute();

            // Reset frame times and perform invalidation loop while atrace is running.
            getActivity().resetFrameTimes();
            latch.await();

            // Copy results.
            final Map<String, Double> metrics =
                    GraphicBufferMetrics.processGraphicBufferResult(output, passTag);
            for (Map.Entry<String, Double> metric : metrics.entrySet()) {
                status.putDouble(metric.getKey(), metric.getValue());
            }
            // Also record FPS.
            status.putDouble(passTag + "_fps", getActivity().getFps());
        }

        getInstrumentation().sendStatus(Activity.RESULT_OK, status);
    }
}

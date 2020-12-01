/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.io.IOException;

import android.app.Activity;
import android.os.Bundle;

// Test that runs GamePerformanceTest with extra CPU load.
public class RenderUnderLoadTest extends RenderTest {
    public void testPerformanceMetricsWithExtraLoad() throws IOException, InterruptedException {
        // Start CPU ballast threads first.
        CPULoadThread[] cpuLoadThreads = new CPULoadThread[2];
        for (int i = 0; i < cpuLoadThreads.length; ++i) {
            cpuLoadThreads[i] = new CPULoadThread();
            cpuLoadThreads[i].start();
        }

        final Bundle status = runPerformanceTests("extra_load_");

        for (int i = 0; i < cpuLoadThreads.length; ++i) {
            cpuLoadThreads[i].issueStopRequest();
            cpuLoadThreads[i].join();
        }

        getInstrumentation().sendStatus(Activity.RESULT_OK, status);
    }
}

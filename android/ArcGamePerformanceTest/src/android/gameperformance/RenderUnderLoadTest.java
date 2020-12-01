/*
 * Copyright (C) 2019 The Android Open Source Project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package android.gameperformance;

import java.io.IOException;

import android.app.Activity;
import android.os.Bundle;
import android.test.ActivityInstrumentationTestCase2;

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
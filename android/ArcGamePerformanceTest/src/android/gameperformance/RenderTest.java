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
import java.util.ArrayList;
import java.util.List;

import android.annotation.NonNull;
import android.app.Activity;
import android.os.Bundle;
import android.test.ActivityInstrumentationTestCase2;

public class RenderTest extends
        ActivityInstrumentationTestCase2<GamePerformanceActivity> {
    private final static int GRAPHIC_BUFFER_WARMUP_LOOP_CNT = 60;

    public RenderTest() {
        super(GamePerformanceActivity.class);
    }

    public void testPerformanceMetrics() throws IOException, InterruptedException {
        final Bundle status = runPerformanceTests("normal_");
        getInstrumentation().sendStatus(Activity.RESULT_OK, status);
    }

    @NonNull
    protected Bundle runPerformanceTests(@NonNull String prefix) {
        final Bundle status = new Bundle();

        final GamePerformanceActivity activity = getActivity();

        final List<BaseTest> tests = new ArrayList<>();
        tests.add(new TriangleCountOpenGLTest(activity));
        tests.add(new FillRateOpenGLTest(activity, false /* testBlend */));
        tests.add(new FillRateOpenGLTest(activity, true /* testBlend */));
        tests.add(new DeviceCallsOpenGLTest(activity));
        tests.add(new ControlsTest(activity));

        for (BaseTest test : tests) {
            final double result = test.run();
            status.putDouble(prefix + test.getName(), result);
        }

        return status;
    }
}
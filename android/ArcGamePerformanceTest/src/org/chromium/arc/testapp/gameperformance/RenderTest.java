/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

import android.app.Activity;
import android.os.Bundle;
import android.test.ActivityInstrumentationTestCase2;

public class RenderTest extends ActivityInstrumentationTestCase2<GamePerformanceActivity> {
    private static final int GRAPHIC_BUFFER_WARMUP_LOOP_CNT = 60;

    public RenderTest() {
        super(GamePerformanceActivity.class);
    }

    public void testPerformanceMetrics() throws IOException, InterruptedException {
        final Bundle status = runPerformanceTests("normal_");
        getInstrumentation().sendStatus(Activity.RESULT_OK, status);
    }

    protected Bundle runPerformanceTests(String prefix) {
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

/*
 * Copyright 2022 The ChromiumOS Authors.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowanimationjank;

import android.os.Bundle;

// import androidx.test.jank.GfxMonitor;
// import androidx.test.jank.JankTest;

/**
 * Detect janks during screen rotation for full-screen activity. Periodically change
 * orientation from left to right and track ElementLayoutActivity rendering performance
 * via GfxMonitor.
 */
// public class FullscreenRotationTest extends WindowAnimationJankTestBase {
//     private final static int STEP_CNT = 3;

    // @Override
    // public void beforeTest() throws Exception {
    //     getUiDevice().setOrientationLeft();
    //     Utils.startElementLayout(getInstrumentation(), 100);
    //     super.beforeTest();
    // }

    // @Override
    // public void afterTest(Bundle metrics) {
    //     Utils.rotateDevice(getInstrumentation(), Utils.ROTATION_MODE_NATURAL);
    //     super.afterTest(metrics);
    // }

    // @JankTest(expectedFrames=100, defaultIterationCount=2)
    // @GfxMonitor(processName=Utils.PACKAGE)
    // public void testRotation() throws Exception {
    //     for (int i = 0; i < STEP_CNT; ++i) {
    //         Utils.rotateDevice(getInstrumentation(),
    //                 Utils.getDeviceRotation(getInstrumentation()) == Utils.ROTATION_MODE_LEFT ?
    //                 Utils.ROTATION_MODE_RIGHT : Utils.ROTATION_MODE_LEFT);
    //     }
    // }
// }

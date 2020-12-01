/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.util.ArrayList;
import java.util.List;

/**
 * Tests that verifies maximum number of device calls to render the geometry to keep FPS close to
 * the device refresh rate. This uses trivial one triangle patch that is rendered multiple times.
 */
public class DeviceCallsOpenGLTest extends RenderPatchOpenGLTest {

    public DeviceCallsOpenGLTest(GamePerformanceActivity activity) {
        super(activity);
    }

    @Override
    public String getName() {
        return "device_calls";
    }

    @Override
    public String getUnitName() {
        return "calls";
    }

    @Override
    public double getUnitScale() {
        return 25.0;
    }

    @Override
    public void initUnits(double deviceCallsD) {
        final List<RenderPatchAnimation> renderPatches = new ArrayList<>();
        final RenderPatch renderPatch =
                new RenderPatch(
                        1 /* triangleCount */,
                        0.05f /* dimension */,
                        RenderPatch.TESSELLATION_BASE);
        final int deviceCalls = (int) Math.round(deviceCallsD);
        for (int i = 0; i < deviceCalls; ++i) {
            renderPatches.add(new RenderPatchAnimation(renderPatch, getView().getRenderRatio()));
        }
        setRenderPatches(renderPatches);
    }
}

/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.util.ArrayList;
import java.util.List;

/**
 * Test that measures maximum amount of triangles can be rasterized keeping FPS close to the device
 * refresh rate. It is has very few devices call and each call contains big amount of triangles.
 * Total filling area is around one screen.
 */
public class TriangleCountOpenGLTest extends RenderPatchOpenGLTest {
    // Based on index buffer of short values.
    private static final int MAX_TRIANGLES_IN_PATCH = 32000;

    public TriangleCountOpenGLTest(GamePerformanceActivity activity) {
        super(activity);
    }

    @Override
    public String getName() {
        return "triangle_count";
    }

    @Override
    public String getUnitName() {
        return "ktriangles";
    }

    @Override
    public double getUnitScale() {
        return 2.0;
    }

    @Override
    public void initUnits(double trianlgeCountD) {
        final int triangleCount = (int) Math.round(trianlgeCountD * 1000.0);
        final List<RenderPatchAnimation> renderPatches = new ArrayList<>();
        final int patchCount =
                (triangleCount + MAX_TRIANGLES_IN_PATCH - 1) / MAX_TRIANGLES_IN_PATCH;
        final int patchTriangleCount = triangleCount / patchCount;
        for (int i = 0; i < patchCount; ++i) {
            final RenderPatch renderPatch =
                    new RenderPatch(
                            patchTriangleCount,
                            0.5f /* dimension */,
                            RenderPatch.TESSELLATION_TO_CENTER);
            renderPatches.add(new RenderPatchAnimation(renderPatch, getView().getRenderRatio()));
        }
        setRenderPatches(renderPatches);
    }
}

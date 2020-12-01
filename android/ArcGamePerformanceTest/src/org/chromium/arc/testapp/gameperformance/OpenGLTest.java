/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import javax.microedition.khronos.opengles.GL;
import javax.microedition.khronos.opengles.GL10;

import org.chromium.arc.testapp.gameperformance.CustomOpenGLView.FrameDrawer;

/** Base class for all OpenGL based tests. */
public abstract class OpenGLTest extends BaseTest {
    public OpenGLTest(GamePerformanceActivity activity) {
        super(activity);
    }

    public CustomOpenGLView getView() {
        return getActivity().getOpenGLView();
    }

    // Performs test drawing.
    protected abstract void draw(GL gl);

    // Initializes the test on first draw call.
    private class ParamFrameDrawer implements FrameDrawer {
        private final double mUnitCount;
        private boolean mInited;

        public ParamFrameDrawer(double unitCount) {
            mUnitCount = unitCount;
            mInited = false;
        }

        @Override
        public void drawFrame(GL10 gl) {
            if (!mInited) {
                initUnits(mUnitCount);
                mInited = true;
            }
            draw(gl);
        }
    }

    @Override
    protected void initProbePass(int probe) {
        try {
            getActivity().attachOpenGLView();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return;
        }
        getView().waitRenderReady();
        getView().setFrameDrawer(new ParamFrameDrawer(probe * getUnitScale()));
    }

    @Override
    protected void freeProbePass() {
        getView().setFrameDrawer(null);
    }
}

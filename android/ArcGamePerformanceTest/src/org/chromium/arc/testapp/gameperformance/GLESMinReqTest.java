/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import android.app.Activity;
import android.content.pm.PackageManager;
import android.opengl.ETC1Util;
import android.os.Bundle;
import android.test.ActivityInstrumentationTestCase2;

import org.chromium.arc.testapp.gameperformance.CustomOpenGLView.FrameDrawer;

import java.io.IOException;
import java.util.concurrent.CountDownLatch;

import javax.microedition.khronos.opengles.GL10;

// Simple class that gathers some OpenGL ES related data, and returns it to the caller.
public class GLESMinReqTest extends ActivityInstrumentationTestCase2<GamePerformanceActivity> {
    // Shared variables between this class, and the inner class that runs in the GL thread.
    private String mGLExtensions = null;
    private String mGLVersion = null;
    private String mGLVendor = null;
    private boolean mSupportsETC1 = false;

    public GLESMinReqTest() {
        super(GamePerformanceActivity.class);
    }

    public void testPerformanceMetrics() throws IOException, InterruptedException {
        final CountDownLatch latch = new CountDownLatch(1);
        final GamePerformanceActivity activity = getActivity();

        // Fetch GL variables from the GL thread
        activity.attachOpenGLView();
        activity.getOpenGLView().waitRenderReady();
        activity.getOpenGLView().setFrameDrawer(new FrameDrawer() {
            @Override
            public void drawFrame(GL10 gl) {
                // Runs in UI thread
                mGLExtensions = gl.glGetString(GL10.GL_EXTENSIONS);
                mGLVersion = gl.glGetString(GL10.GL_VERSION);
                mGLVendor = gl.glGetString(GL10.GL_VENDOR);

                mSupportsETC1 = ETC1Util.isETC1Supported();

                // Remove FrameDrawer, since this is a one-time only thing
                activity.getOpenGLView().setFrameDrawer(null);

                latch.countDown();
            }
        });

        latch.await();

        final boolean deviceSupportsAEP = activity.getPackageManager().hasSystemFeature(
                PackageManager.FEATURE_OPENGLES_EXTENSION_PACK);

        final Bundle status = new Bundle();
        status.putString("gl_version", mGLVersion);
        status.putString("gl_vendor", mGLVendor);
        status.putString("gl_extensions", mGLExtensions);
        status.putBoolean("supports_AEP", deviceSupportsAEP);
        status.putBoolean("supports_ETC1", mSupportsETC1);
        getInstrumentation().sendStatus(Activity.RESULT_OK, status);
    }
}

/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.util.ArrayList;
import java.util.List;

import javax.microedition.khronos.egl.EGLConfig;
import javax.microedition.khronos.opengles.GL10;

import android.content.Context;
import android.opengl.GLES20;
import android.opengl.GLSurfaceView;
import android.util.Log;

public class CustomOpenGLView extends GLSurfaceView {
    public static final String TAG = "CustomOpenGLView";

    private final List<Long> mFrameTimes;
    private final Object mLock = new Object();
    private boolean mRenderReady = false;
    private FrameDrawer mFrameDrawer = null;

    private float mRenderRatio;
    private int mRenderWidth;
    private int mRenderHeight;

    public interface FrameDrawer {
        public void drawFrame(GL10 gl);
    }

    public CustomOpenGLView(Context context) {
        super(context);

        mFrameTimes = new ArrayList<Long>();

        setEGLContextClientVersion(2);

        setRenderer(
                new GLSurfaceView.Renderer() {
                    @Override
                    public void onSurfaceCreated(GL10 gl, EGLConfig config) {
                        Log.i(TAG, "SurfaceCreated: " + config);
                        GLES20.glClearColor(1.0f, 0.0f, 0.0f, 1.0f);
                        gl.glClearDepthf(1.0f);
                        gl.glDisable(GL10.GL_DEPTH_TEST);
                        gl.glDepthFunc(GL10.GL_LEQUAL);

                        gl.glHint(GL10.GL_PERSPECTIVE_CORRECTION_HINT, GL10.GL_NICEST);
                        synchronized (mLock) {
                            mRenderReady = true;
                            mLock.notify();
                        }
                    }

                    @Override
                    public void onSurfaceChanged(GL10 gl, int width, int height) {
                        Log.i(TAG, "SurfaceChanged: " + width + "x" + height);
                        GLES20.glViewport(0, 0, width, height);
                        setRenderBounds(width, height);
                    }

                    @Override
                    public void onDrawFrame(GL10 gl) {
                        GLES20.glClearColor(0.25f, 0.25f, 0.25f, 1.0f);
                        gl.glClear(GL10.GL_COLOR_BUFFER_BIT | GL10.GL_DEPTH_BUFFER_BIT);
                        synchronized (mLock) {
                            if (mFrameDrawer != null) {
                                mFrameDrawer.drawFrame(gl);
                            }
                            mFrameTimes.add(System.currentTimeMillis());
                        }
                    }
                });
        setRenderMode(GLSurfaceView.RENDERMODE_CONTINUOUSLY);
    }

    public void setRenderBounds(int width, int height) {
        mRenderWidth = width;
        mRenderHeight = height;
        mRenderRatio = (float) mRenderWidth / mRenderHeight;
    }

    public float getRenderRatio() {
        return mRenderRatio;
    }

    public int getRenderWidth() {
        return mRenderWidth;
    }

    public int getRenderHeight() {
        return mRenderHeight;
    }

    /** Resets frame times in order to calculate FPS for the different test pass. */
    public void resetFrameTimes() {
        synchronized (mLock) {
            mFrameTimes.clear();
        }
    }

    /** Returns current FPS based on collected frame times. */
    public double getFps() {
        synchronized (mLock) {
            if (mFrameTimes.size() < 2) {
                return 0.0f;
            }
            return 1000.0
                    * mFrameTimes.size()
                    / (mFrameTimes.get(mFrameTimes.size() - 1) - mFrameTimes.get(0));
        }
    }

    /** Waits for render attached to the view. */
    public void waitRenderReady() {
        synchronized (mLock) {
            while (!mRenderReady) {
                try {
                    mLock.wait();
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            }
        }
    }

    /** Sets/resets frame drawer. */
    public void setFrameDrawer(FrameDrawer frameDrawer) {
        mFrameDrawer = frameDrawer;
    }
}

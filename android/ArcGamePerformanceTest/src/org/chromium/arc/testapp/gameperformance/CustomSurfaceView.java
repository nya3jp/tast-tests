/*
 * Copyright 2020 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.util.ArrayList;
import java.util.List;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.PixelFormat;
import android.os.Handler;
import android.os.HandlerThread;
import android.os.Trace;
import android.view.Surface;
import android.view.SurfaceHolder;
import android.view.SurfaceView;

/** Minimal SurfaceView that sends buffer on request. */
public class CustomSurfaceView extends SurfaceView implements SurfaceHolder.Callback {
    // Tag for trace when buffer is requested.
    public static final String LOCAL_REQUEST_BUFFER = "localRequestBuffer";
    // Tag for trace when buffer is posted.
    public static final String LOCAL_POST_BUFFER = "localPostBuffer";

    private final Object mSurfaceLock = new Object();
    // Keeps frame times. Used to calculate fps.
    private List<Long> mFrameTimes;
    // Surface to send.
    private Surface mSurface;
    private Handler mHandler;

    private Runnable mInvalidateSurfaceTask =
            new Runnable() {
                @Override
                public void run() {
                    synchronized (mSurfaceLock) {
                        if (mSurface == null) {
                            return;
                        }
                        invalidateSurface(true, true);
                        mHandler.post(this);
                    }
                }
            };

    public CustomSurfaceView(Context context) {
        super(context);
        mFrameTimes = new ArrayList<Long>();
        getHolder().addCallback(this);
        getHolder().setFormat(PixelFormat.OPAQUE);

        HandlerThread thread = new HandlerThread("SurfaceInvalidator");
        thread.start();
        mHandler = new Handler(thread.getLooper());
    }

    /** Resets frame times in order to calculate fps for different test pass. */
    public void resetFrameTimes() {
        synchronized (mSurfaceLock) {
            mFrameTimes.clear();
        }
    }

    /** Returns current fps based on collected frame times. */
    public double getFps() {
        synchronized (mSurfaceLock) {
            if (mFrameTimes.size() < 2) {
                return 0.0f;
            }
            return 1000.0
                    * mFrameTimes.size()
                    / (mFrameTimes.get(mFrameTimes.size() - 1) - mFrameTimes.get(0));
        }
    }

    /**
     * Invalidates surface.
     *
     * @param traceCalls set to true in case we need register trace calls. Not used for warm-up.
     * @param drawFps perform drawing current fps on surface to have some payload on surface.
     */
    public void invalidateSurface(boolean traceCalls, boolean drawFps) {
        synchronized (mSurfaceLock) {
            if (mSurface == null) {
                throw new IllegalStateException("Surface is not ready");
            }
            if (traceCalls) {
                Trace.beginSection(LOCAL_REQUEST_BUFFER);
            }
            Canvas canvas = mSurface.lockHardwareCanvas();
            if (traceCalls) {
                Trace.endSection();
            }

            if (drawFps) {
                int textSize = canvas.getHeight() / 24;
                Paint paint = new Paint();
                paint.setTextSize(textSize);
                paint.setColor(0xFFFF8040);
                canvas.drawARGB(92, 255, 255, 255);
                canvas.drawText("FPS: " + String.format("%.2f", getFps()), 10, 300, paint);
            }

            if (traceCalls) {
                Trace.beginSection(LOCAL_POST_BUFFER);
            }
            mSurface.unlockCanvasAndPost(canvas);
            if (traceCalls) {
                Trace.endSection();
            }

            mFrameTimes.add(System.currentTimeMillis());
        }
    }

    /**
     * Wait until surface is created and ready to use or return immediately if surface already
     * exists.
     */
    public void waitForSurfaceReady() {
        synchronized (mSurfaceLock) {
            if (mSurface == null) {
                try {
                    mSurfaceLock.wait(5000);
                } catch (InterruptedException e) {
                    e.printStackTrace();
                }
            }
            if (mSurface == null) throw new IllegalStateException("Surface is not ready.");
        }
    }

    /** Waits until surface is destroyed or return immediately if surface does not exist. */
    public void waitForSurfaceDestroyed() {
        synchronized (mSurfaceLock) {
            if (mSurface != null) {
                try {
                    mSurfaceLock.wait(5000);
                } catch (InterruptedException e) {
                }
            }
            if (mSurface != null) throw new IllegalStateException("Surface still exists.");
        }
    }

    @Override
    public void surfaceCreated(SurfaceHolder holder) {}

    @Override
    public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {
        // This method is always called at least once, after surfaceCreated.
        synchronized (mSurfaceLock) {
            mSurface = holder.getSurface();
            mSurfaceLock.notify();
            mHandler.post(mInvalidateSurfaceTask);
        }
    }

    @Override
    public void surfaceDestroyed(SurfaceHolder holder) {
        synchronized (mSurfaceLock) {
            mHandler.removeCallbacks(mInvalidateSurfaceTask);
            mSurface = null;
            mSurfaceLock.notify();
        }
    }
}

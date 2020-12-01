/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;

import android.app.Activity;
import android.content.Context;
import android.graphics.Canvas;
import android.graphics.drawable.AnimationDrawable;
import android.view.WindowManager;
import android.widget.AbsoluteLayout;
import android.widget.ImageView;

/** View that holds requested number of UI controls as ImageView with an infinite animation. */
public class CustomControlView extends AbsoluteLayout {
    private static final int CONTROL_DIMENTION = 48;

    private final int mPerRowControlCount;
    private List<Long> mFrameTimes = new ArrayList<>();

    public CustomControlView(Context context) {
        super(context);

        final WindowManager windowManager =
                (WindowManager) context.getSystemService(Context.WINDOW_SERVICE);
        mPerRowControlCount = windowManager.getDefaultDisplay().getWidth() / CONTROL_DIMENTION;
    }

    /**
     * Helper class that overrides ImageView and observes draw requests. Only one such control is
     * created which is the first control in the view.
     */
    class ReferenceImageView extends ImageView {
        public ReferenceImageView(Context context) {
            super(context);
        }

        @Override
        public void draw(Canvas canvas) {
            reportFrame();
            super.draw(canvas);
        }
    }

    public void createControls(Activity activity, int controlCount) throws InterruptedException {
        synchronized (this) {
            final CountDownLatch latch = new CountDownLatch(1);
            activity.runOnUiThread(
                    new Runnable() {
                        @Override
                        public void run() {
                            removeAllViews();

                            for (int i = 0; i < controlCount; ++i) {
                                final ImageView image =
                                        (i == 0)
                                                ? new ReferenceImageView(activity)
                                                : new ImageView(activity);
                                final int x = (i % mPerRowControlCount) * CONTROL_DIMENTION;
                                final int y = (i / mPerRowControlCount) * CONTROL_DIMENTION;
                                final AbsoluteLayout.LayoutParams layoutParams =
                                        new AbsoluteLayout.LayoutParams(
                                                CONTROL_DIMENTION, CONTROL_DIMENTION, x, y);
                                image.setLayoutParams(layoutParams);
                                image.setBackgroundResource(R.drawable.animation);
                                final AnimationDrawable animation =
                                        (AnimationDrawable) image.getBackground();
                                animation.start();
                                addView(image);
                            }

                            latch.countDown();
                        }
                    });
            latch.await();
        }
    }

    private void reportFrame() {
        final long time = System.currentTimeMillis();
        synchronized (mFrameTimes) {
            mFrameTimes.add(time);
        }
    }

    /** Resets frame times in order to calculate FPS for the different test pass. */
    public void resetFrameTimes() {
        synchronized (mFrameTimes) {
            mFrameTimes.clear();
        }
    }

    /** Returns current FPS based on collected frame times. */
    public double getFps() {
        synchronized (mFrameTimes) {
            if (mFrameTimes.size() < 2) {
                return 0.0f;
            }
            return 1000.0
                    * mFrameTimes.size()
                    / (mFrameTimes.get(mFrameTimes.size() - 1) - mFrameTimes.get(0));
        }
    }
}

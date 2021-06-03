/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */


package org.chromium.arc.testapp.arcblackflashtest;

import android.app.Activity;
import android.graphics.Point;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.view.WindowMetrics;

/**
 * Test Activity that can show a black flash when maximized. The arc.BlackFlash test
 * restores it and maximizes it to check if any black-flash appears during those
 * state transitions.
 */
public class MainActivity extends Activity {

    // Note if we block the thread for more than 5 seconds, the Framework can throw ANR.
    private final int BLACK_FLASH_DURATION_MS = 3000;

    private int mPrevWidth = Integer.MAX_VALUE;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        if (savedInstanceState != null) {
            mPrevWidth = savedInstanceState.getInt("PrevWidth");
        }

        final int currentWidth = getCurrentWindowWidth();
        // Checking the size of the window should be enough to see if the app transitioned from
        // restored to maximized.
        if (currentWidth > mPrevWidth) {
            try {
                // We need to block the UI thread to show black flashes.
                Thread.sleep(BLACK_FLASH_DURATION_MS);
            } catch (InterruptedException e) {
                Log.e("BlackFlashApp", e.toString());
            }
            // This layout has a blue background.
            setContentView(R.layout.maximized);
        } else {
            // This layout has a white background.
            setContentView(R.layout.main_activity);
        }

        mPrevWidth = currentWidth;
    }

    @Override
    protected void onSaveInstanceState(Bundle savedInstanceState) {
        super.onSaveInstanceState(savedInstanceState);
        savedInstanceState.putInt("PrevWidth", mPrevWidth);
    }

    int getCurrentWindowWidth() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            WindowMetrics windowMetrics = getWindowManager().getCurrentWindowMetrics();
            return windowMetrics.getBounds().width();
        }

        // Prior to R, there is no canonical API to get the width of the window.
        // Display#getSize() returns the app's window size if it's called with the activity context.
        // Please refer to https://developer.android.com/reference/android/view/Display#getSize(android.graphics.Point).
        Point size = new Point();
        getWindowManager().getDefaultDisplay().getSize(size);
        return size.x;
    }
}

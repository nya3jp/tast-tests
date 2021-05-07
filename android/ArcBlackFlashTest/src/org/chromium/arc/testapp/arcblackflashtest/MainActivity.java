/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */


package org.chromium.arc.testapp.arcblackflashtest;

import android.app.Activity;
import android.os.Bundle;
import android.util.Log;
import android.view.WindowMetrics;

/**
 * Test Activity that shows a black flash when maximized. The arc.BlackFlash test launches this app
 * in maximized state, restores it and maximizes it to check if blackflashes appear during those
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
        final WindowMetrics windowMetrics = getWindowManager().getCurrentWindowMetrics();
        if (windowMetrics.getBounds().width() > mPrevWidth) {
            try {
                // We need to block the UI thread to show black flashes.
                Thread.sleep(BLACK_FLASH_DURATION_MS);
            } catch (InterruptedException e) {
                Log.e("BlackFlashApp", e.toString());
            }
            setContentView(R.layout.maximized);
        } else {
            setContentView(R.layout.main_activity);
        }

        mPrevWidth = windowMetrics.getBounds().width();
    }

    @Override
    protected void onSaveInstanceState(Bundle savedInstanceState) {
        super.onSaveInstanceState(savedInstanceState);
        WindowMetrics windowMetrics = getWindowManager().getCurrentWindowMetrics();
        savedInstanceState.putInt("PrevWidth", windowMetrics.getBounds().width());
    }
}

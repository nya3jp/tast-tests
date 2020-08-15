/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.animationperformance;

import java.util.*;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.util.Log;

import android.graphics.Color;
import android.widget.GridLayout;
import android.widget.ImageView;
import android.transition.AutoTransition;
import android.transition.TransitionManager;


public class MainActivity extends Activity {

    GridLayout mGrid;

    // arc.RuntimePerf test waits for this string to be logged on app load.
    private static final String LOG_START = "com.android.game.qualification.START";
    private static final String TAG = "MainActivity";

    private static final int SHUFFLE_PERIOD_MS = 300;

    private static final int COLS = 35;
    private static final int ROWS = 20;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        Random rand = new Random(1453);

        mGrid = findViewById(R.id.grid);

        mGrid.setColumnCount(COLS);
        mGrid.setRowCount(ROWS);

        int count = COLS*ROWS;

        for(int i = 0 ; i < count ; i++) {
            ImageView imageView = new ImageView(this);

            // A full cycle of colors with 0.9 value.
            float hue = 360f * i / count;
            imageView.setBackgroundColor(Color.HSVToColor(new float[]{hue, 1, 0.9f}));

            // Set both column and row weights as 1 so that all children fill evenly.
            GridLayout.LayoutParams params = new GridLayout.LayoutParams();
            params.columnSpec = GridLayout.spec(GridLayout.UNDEFINED, 1f);
            params.rowSpec = GridLayout.spec(GridLayout.UNDEFINED, 1f);
            imageView.setLayoutParams(params);

            mGrid.addView(imageView);
        }

        // Setup animation loop.
        Handler handler = new Handler();
        Runnable runnable = new Runnable() {
            @Override
            public void run() {
                TransitionManager.beginDelayedTransition(mGrid, new AutoTransition());

                // Remove a random view and add it back to the end for a pseudo-shuffle effect.
                for(int i  = 0 ; i < count; i++) {
                    ImageView imageView = ((ImageView)mGrid.getChildAt(rand.nextInt(count)));
                    mGrid.removeView(imageView);
                    mGrid.addView(imageView);
                }

                handler.postDelayed(this, SHUFFLE_PERIOD_MS);
            }
        };

        // Start loop immediately for a more accurate load time.
        handler.post(runnable);

        // Signal app loaded.
        Log.v(TAG, LOG_START);
    }
}

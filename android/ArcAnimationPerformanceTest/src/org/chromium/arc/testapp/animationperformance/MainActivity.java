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
import android.transition.Transition.TransitionListener;
import android.transition.Transition;


public class MainActivity extends Activity {

    // arc.RuntimePerf test waits for this string to be logged on app load.
    private static final String LOG_START = "com.android.game.qualification.START";
    private static final String TAG = "MainActivity";

    private static final int COLS = 35;
    private static final int ROWS = 20;

    private GridLayout mGrid;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        final Random rand = new Random(1453);

        mGrid = findViewById(R.id.grid);

        mGrid.setColumnCount(COLS);
        mGrid.setRowCount(ROWS);

        final int count = COLS*ROWS;

        for(int i = 0 ; i < count ; i++) {
            ImageView imageView = new ImageView(this);

            // A full cycle of colors with 0.9 value.
            final float hue = 360f * i / count;
            imageView.setBackgroundColor(Color.HSVToColor(new float[]{hue, 1, 0.9f}));

            // Set both column and row weights as 1 so that all children fill evenly.
            final GridLayout.LayoutParams params = new GridLayout.LayoutParams();
            params.columnSpec = GridLayout.spec(GridLayout.UNDEFINED, 1f);
            params.rowSpec = GridLayout.spec(GridLayout.UNDEFINED, 1f);
            imageView.setLayoutParams(params);

            mGrid.addView(imageView);
        }

        // Setup shuffle loop.
        final Handler handler = new Handler();
        final Runnable shuffle = new Runnable() {
            @Override
            public void run() {
                // 'this' is confused with TransitionListener class when used below
                final Runnable shuffle = this;

                final AutoTransition transition = new AutoTransition();
                final TransitionListener listener = new TransitionListener() {
                    @Override
                    public void onTransitionEnd(Transition transition) {
                        handler.post(shuffle);
                    }

                    @Override
                    public void onTransitionStart(Transition transition) {}

                    @Override
                    public void onTransitionPause(Transition transition) {}

                    @Override
                    public void onTransitionResume(Transition transition) {}

                    @Override
                    public void onTransitionCancel(Transition transition) {}
                };

                transition.addListener(listener);

                TransitionManager.beginDelayedTransition(mGrid, transition);

                // Remove a random view and add it back to the end for a pseudo-shuffle effect.
                for(int i  = 0 ; i < count; i++) {
                    final ImageView imageView = ((ImageView)mGrid.getChildAt(rand.nextInt(count)));
                    mGrid.removeView(imageView);
                    mGrid.addView(imageView);
                }
            }
        };

        // First shuffle has happen after the grid is created
        // so that TransitionManager can detect a change.
        mGrid.post(shuffle);

        // Signal app loaded.
        Log.v(TAG, LOG_START);
    }
}

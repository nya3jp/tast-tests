/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedtouchscreentest;

import android.app.Activity;
import android.os.Bundle;
import android.view.MotionEvent;
import android.view.View;
import android.view.View.OnTouchListener;
import android.widget.TextView;

import java.util.HashSet;
import java.util.Set;

public class ThreeFingerSwipeTestActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_three_finger_swipe_test);

        // Detect three finger swipes on the element.
        TextView txtThreeFingerSwipe = findViewById(R.id.txtThreeFingerSwipe);
        txtThreeFingerSwipe.setOnTouchListener(
                new MultiFingerSwipeGestureDetector(
                        3,
                        (direction, distance) -> {
                            // Print the last swipe direction.
                            TextView txtTestState = findViewById(R.id.txtTestState);
                            txtTestState.setText(
                                    String.format(
                                            "Direction: %s, Distance: %s",
                                            direction.name(), distance));
                        }));
    }

    /** Fired when a multi-finger swipe is detected. */
    private interface MultiFingerSwipeListener {
        void onMultiFingerSwipe(MultiFingerSwipeDirection direction, int distance);
    }

    /** Used to determine the direction of a swipe. */
    private enum MultiFingerSwipeDirection {
        UP,
        DOWN,
        LEFT,
        RIGHT
    }

    /** Holds values of a point. */
    private class Point {

        private final int x;
        private final int y;

        public Point(int x, int y) {
            this.x = x;
            this.y = y;
        }
    }

    /** Detects multi-finger swipes and alerts the listener. */
    private class MultiFingerSwipeGestureDetector implements OnTouchListener {
        private static final int INVALID_POINTER_ID = -1;

        private final MultiFingerSwipeListener mOnMultiFingerSwipeListener;
        private final int mRequiredNumTouches;
        private int mInitialPointerId;
        private Point mStartPoint;
        private Point mEndPoint;
        private Set<Integer> mTrackedPointers;

        MultiFingerSwipeGestureDetector(
                int requiredNumTouches, MultiFingerSwipeListener onMultiFingerSwipeListener) {
            mOnMultiFingerSwipeListener = onMultiFingerSwipeListener;
            mRequiredNumTouches = requiredNumTouches;
            this.resetState();
        }

        private void resetState() {
            mInitialPointerId = INVALID_POINTER_ID;
            mStartPoint = null;
            mEndPoint = null;
            mTrackedPointers = null;
        }

        @Override
        public boolean onTouch(View v, MotionEvent event) {
            switch (event.getActionMasked()) {
                case MotionEvent.ACTION_MOVE:
                    {
                        // Update the end position if the move happened for the initial pointer.
                        int pointerIndex = event.findPointerIndex(mInitialPointerId);
                        if (pointerIndex > -1) {
                            mEndPoint =
                                    new Point(
                                            (int) event.getX(pointerIndex),
                                            (int) event.getY(pointerIndex));
                        }
                        break;
                    }
                case MotionEvent.ACTION_DOWN:
                    {
                        // The first down event will trigger the tracking logic.
                        mInitialPointerId = event.getPointerId(0);
                        mStartPoint =
                                new Point(
                                        (int) event.getX(mInitialPointerId),
                                        (int) event.getY(mInitialPointerId));
                        mTrackedPointers = new HashSet<>();
                        mTrackedPointers.add(mInitialPointerId);
                        break;
                    }
                case MotionEvent.ACTION_POINTER_DOWN:
                    {
                        // All other pointers just add to the list of tracked pointers.
                        int pointerId = event.getPointerId(event.getActionIndex());
                        mTrackedPointers.add(pointerId);
                        break;
                    }
                case MotionEvent.ACTION_UP:
                    {
                        // Determine if the event should fire after the last pointer is removed.
                        int dx = mEndPoint.x - mStartPoint.x;
                        int dy = mEndPoint.y - mStartPoint.y;
                        boolean wasMoved = dx != 0 || dy != 0;
                        boolean usedRequiredNumPointers =
                                mTrackedPointers.size() == mRequiredNumTouches;
                        if (wasMoved && usedRequiredNumPointers) {
                            // Determine the direction of the swipe, determined by which axis had
                            // the most movement.
                            if (Math.abs(dx) >= Math.abs(dy)) {
                                mOnMultiFingerSwipeListener.onMultiFingerSwipe(
                                        dx >= 0
                                                ? MultiFingerSwipeDirection.RIGHT
                                                : MultiFingerSwipeDirection.LEFT,
                                        Math.abs(dx));
                            } else {
                                mOnMultiFingerSwipeListener.onMultiFingerSwipe(
                                        dy >= 0
                                                ? MultiFingerSwipeDirection.DOWN
                                                : MultiFingerSwipeDirection.UP,
                                        Math.abs(dy));
                            }
                        }

                        // Always reset the state.
                        this.resetState();

                        break;
                    }
            }

            return true;
        }
    }
}

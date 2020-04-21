/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.quartersizedwindowzoomingtest;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Rect;
import android.view.View;

// In this view class, each row in the display pixels are painted
// in black and white alternately.
public class StripeView extends View {
    private Paint mPaint = new Paint();
    private Rect mBounds = new Rect();

    private void init() {
        mPaint.setStrokeWidth(0); // hairline mode
        mPaint.setColor(Color.BLACK);
    }

    public StripeView(Context context) {
        super(context);
        init();
    }

    @Override
    public void onDraw(Canvas canvas) {
        for (int i = 0; i < mBounds.bottom; i++) {
            if (i % 2 == 0) {
                canvas.drawLine(0, i, mBounds.right, i, mPaint);
            }
        }
    }

    @Override
    protected void onSizeChanged(int w, int h, int oldw, int oldh) {
        mBounds.set(0, 0, w, h);
        invalidate();
    }
}

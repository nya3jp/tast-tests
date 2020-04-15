/*
 * Copyright (C) 2019 The Android Open Source Project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
        for (int i = 0; i <= mBounds.bottom; i++) {
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

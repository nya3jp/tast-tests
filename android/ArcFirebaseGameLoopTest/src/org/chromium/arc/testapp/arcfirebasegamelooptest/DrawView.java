/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */
package org.chromium.arc.testapp.arcfirebasegamelooptest;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.view.View;

public class DrawView  extends View{
  Paint paint = new Paint();

  int size = 100;
  float x = 0;
  float y = 0;

  public DrawView(Context context) {
    super(context);
    paint.setColor(Color.RED);

  }

  @Override
  protected void onDraw(Canvas canvas) {
    super.onDraw(canvas);
    canvas.drawRect(x, y, x + size, y+size, paint);
  }

  public void Move(float dx, float dy){
    x += dx;
    y += dy;
  }

}

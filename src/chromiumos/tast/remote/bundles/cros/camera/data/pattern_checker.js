// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

class PatternChecker{
   /*
   * Checks the |aspectRatio| camera FOV of |facing| camera is aligned with
   * pattern.
   * @param {ImageData} imageData
   * @return {!Promise<boolean>}
   * @private
   */
 static checkAlign(pattern,imageData,canvas_match=null,prop=1) {

    const h_boundCoef0 = 0.1, h_boundCoef1 = 0.30;
    const v_boundCoef0 = 0.1, v_boundCoef1 = 0.30;
    const MIN_MATCH_COUNT = 20;

    let im1 = cv.imread(pattern), im2 = cv.matFromImageData(imageData);
    let im1Gray = new cv.Mat(), im2Gray = new cv.Mat();
    let kp1 = new cv.KeyPointVector(), kp2 = new cv.KeyPointVector();
    let des1 = new cv.Mat(), des2 = new cv.Mat();
    let sift = new cv.SIFT(), flann = new cv.FlannBasedMatcher()
    let good_matches = new cv.DMatchVector();
    let matches = new cv.DMatchVectorVector();
    let imMatches = new cv.Mat(), imMatches_resized = new cv.Mat();
    let dts = new cv.Mat(), markersVector = new cv.MatVector();
    const mv = new cv.Mat(4, 1, cv.CV_32SC2);
    cv.cvtColor(im1, im1Gray, cv.COLOR_BGRA2GRAY);
    cv.cvtColor(im2, im2Gray, cv.COLOR_BGRA2GRAY);
    sift.detectAndCompute(im1Gray, new cv.Mat(), kp1, des1);
    sift.detectAndCompute(im2Gray, new cv.Mat(), kp2, des2);
    flann.knnMatch(des1, des2, matches, 2);
    for (let i = 0; i < matches.size(); ++i) {
        let match = matches.get(i);
        let dMatch1 = match.get(0);
        let dMatch2 = match.get(1);
        if (dMatch1.distance < dMatch2.distance * 2) {
            good_matches.push_back(dMatch1);
        }
    }
    if (good_matches.size() < MIN_MATCH_COUNT){
      return false;
    }
    let points1 = [], points2 = [];
    for (let i = 0; i < good_matches.size(); i++) {
        points1.push(kp1.get(good_matches.get(i).queryIdx ).pt.x);
        points2.push(kp2.get(good_matches.get(i).trainIdx ).pt.x );
        points1.push(kp1.get(good_matches.get(i).queryIdx ).pt.y );
        points2.push(kp2.get(good_matches.get(i).trainIdx ).pt.y );
    }
    let mat1 = cv.matFromArray(points1.length/2, 1, cv.CV_64FC2, points1);
    let mat2 = cv.matFromArray(points2.length/2, 1, cv.CV_64FC2, points2);
    let h = cv.findHomography(mat1, mat2, cv.RANSAC,5.0);
    let width = im1.size().width,height = im1.size().height;
    if (h.empty())
    {
      return false;
    }
    let pts_tmp = [0,0,
                  0,height-1,
                  width-1,height-1,
                  width-1,0];
    let pts = cv.matFromArray(4,1, cv.CV_32FC2, pts_tmp);
    cv.perspectiveTransform(pts,dts,h)
    dts.convertTo(mv,cv.CV_32SC2);

    let p = [ [dts.floatPtr(0, 0)[0],dts.floatPtr(0, 0)[1]],
              [dts.floatPtr(1, 0)[0],dts.floatPtr(1, 0)[1]],
              [dts.floatPtr(2, 0)[0],dts.floatPtr(2, 0)[1]],
              [dts.floatPtr(3, 0)[0],dts.floatPtr(3, 0)[1]],]
    width = im2.size().width,height = im2.size().height;
    let h_bound = [width*h_boundCoef0*-1, width*h_boundCoef1,
                   width*(1-h_boundCoef1),  width*(1+h_boundCoef0) ];
    let v_bound = [height*v_boundCoef0*-1,height*v_boundCoef1,
                   height*(1-v_boundCoef1), height*(1+v_boundCoef0)];
    let bound = [[h_bound[0],h_bound[1],v_bound[0],v_bound[1]],
                 [h_bound[0],h_bound[1],v_bound[2],v_bound[3]],
                 [h_bound[2],h_bound[3],v_bound[2],v_bound[3]],
                 [h_bound[2],h_bound[3],v_bound[0],v_bound[1]]];
    if (canvas_match){

      markersVector.push_back(mv);
      cv.polylines(im2, markersVector, true, new cv.Scalar(255,0,0),3);
      markersVector = new cv.MatVector();
      let area = new cv.matFromArray(4,1,cv.CV_32SC2,
            [h_bound[0],v_bound[0],h_bound[0],v_bound[1],
            h_bound[1],v_bound[1],h_bound[1],v_bound[0]]);
      markersVector.push_back(area );
      area = new cv.matFromArray(4,1,cv.CV_32SC2,
            [h_bound[2],v_bound[0],h_bound[2],v_bound[1],
            h_bound[3],v_bound[1],h_bound[3],v_bound[0]]);
      markersVector.push_back(area );
      area = new cv.matFromArray(4,1,cv.CV_32SC2,
            [h_bound[2],v_bound[2],h_bound[2],v_bound[3],
            h_bound[3],v_bound[3],h_bound[3],v_bound[2]]);
      markersVector.push_back(area );
      area = new cv.matFromArray(4,1,cv.CV_32SC2,
            [h_bound[0],v_bound[2],h_bound[0],v_bound[3],
            h_bound[1],v_bound[3],h_bound[1],v_bound[2]]);
      markersVector.push_back(area );
      cv.polylines(im2, markersVector, true, [255, 0, 0, 255],2,4);
      cv.drawMatches(im1, kp1, im2, kp2,
                     good_matches, imMatches,
                     new cv.Scalar(0,255,0, 255));
      let dsize = new cv.Size(Math.ceil(imMatches.size().width*prop),
                              Math.ceil(imMatches.size().height*prop));
      cv.resize(imMatches, imMatches_resized, dsize, 0, 0, cv.INTER_AREA);
      cv.imshow(canvas_match, imMatches_resized);
    }
    let normal = true;
    for (let i =0;i<4;i++){
      let x = parseInt(p[i][0]), y = parseInt(p[i][1]);
      if (x < bound[i][0] || x > bound[i][1] ||
          y < bound[i][2] || y > bound[i][3] ){
        normal = false;
      }
    }
    let rotate = true;
    for (let i =0;i<4;i++){
      let x = parseInt(p[(i+2)%4][0]), y = parseInt(p[(i+2)%4][1]);
      if (x < bound[i][0] || x > bound[i][1] ||
          y < bound[i][2] || y > bound[i][3] ){
        rotate = false;
      }
    }
    if (! (normal || rotate)){
      console.log(bound)
    }
    return normal || rotate;
   }
}
exports = {PatternChecker};
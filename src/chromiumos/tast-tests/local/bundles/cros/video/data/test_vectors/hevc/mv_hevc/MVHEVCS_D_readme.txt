Test bitstream #MVHEVCS-D

Specification: All slices are coded as I, P or B slices. Only the first picture of each view is coded as an IDR picture. Each picture contains only one slice. NumViews is equal to 2. NumDirectRefLayers is always equal to 0, meaning inter-view prediction is disabled. The two views are with different spatial resolutions. All NAL units are encapsulated into the byte stream format specified in Rec. ITU-T H.265 | ISO/IEC 23008-2.
Functional stage: Decoding of two views with only inter-prediction and intra-prediction.
Purpose: To conform the case when different spatial resolutions for two views are used.

resolution:     1024x768 (View0), 512x384 (View1)
# of views:     2 Texture
# of frames:    25
Coding Structure(time): IBP
Coding Structure(view): I
Bitstream name: MVHEVCS_D.hevc

Software: HTM-15.1

Configurations:
#======== Coding Structure =============
IntraPeriod                   : -1          # Period of I-Frame ( -1 = only first)
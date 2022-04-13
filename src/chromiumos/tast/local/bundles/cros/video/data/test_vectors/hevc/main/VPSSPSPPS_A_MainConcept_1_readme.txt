file_name: VPSSPSPPS_A_MainConcept_1.hevc
resolution & level: main 6.2, Chroma format 4:2:0
Frame:      0 POC:     0, SLICE_TYPE: I 176x144
Frame:      1 POC:     0, SLICE_TYPE: I 320x240
Frame:      2 POC:     0, SLICE_TYPE: I 352x288
Frame:      3 POC:     0, SLICE_TYPE: I 640x480
Frame:      4 POC:     0, SLICE_TYPE: I 704x576
Frame:      5 POC:     0, SLICE_TYPE: I 1280x720
frame-rate: 29.97
length: 6 frames
features:
The stream excercises conformance with SPS, VPS, and PPS. The stream contains only 6 I frames, each of them with a different resolution. Each frame has parameters sets with a unique ID:
frame 0, VPS_ID = 1, SPS_ID = 3, PPS_ID = 2
frame 1, VPS_ID = 3, SPS_ID = 4, PPS_ID = 1
frame 2, VPS_ID = 2, SPS_ID = 5, PPS_ID = 6
frame 3, VPS_ID = 4, SPS_ID = 1, PPS_ID = 3
frame 4, VPS_ID = 5, SPS_ID = 2, PPS_ID = 4
frame 5, VPS_ID = 6, SPS_ID = 6, PPS_ID = 5
All headers are at the beginning of the file (prior to any frames), some of the headers are duplicated. The order of the headers is arbitrary: VPS_NUT, PPS_NUT, PPS_NUT, VPS_NUT, VPS_NUT, VPS_NUT, VPS_NUT, VPS_NUT, SPS_NUT, VPS_NUT, ...

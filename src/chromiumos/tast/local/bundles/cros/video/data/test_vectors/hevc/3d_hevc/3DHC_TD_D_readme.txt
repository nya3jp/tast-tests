-Bitstream file:   3DHC_TD_D.hevc
-MD5 file:         3DHC_TD_D.md5
-Specification:    The bitstream is coded with random access configuration as well as 3-view configuration.
                   In addition to all the texture coding tools, all the depth coding tools and depth dependent texture coding tools, Intra-view Contour Mode (DMM4) is turned on.
-Functional stage: Decoding of three views with Intra-view Contour Mode enabled for depth views.
-Purpose:          To conform the Intra-view Contour Mode for depth with random access configuration.
-Software:         HTM-15.1
-Configuration:    (based on random access default configuration)
                   #======== VPS / PTLI ================
                   Profile                             : main main 3d-main
                   Level                               : 5.1  5.1  5.1
                   Tier                                : main main main
                   InblFlag                            : 0    0    0
                   #========== depth coding tools ==========
                   VSO                                 : 1
                   IntraWedgeFlag                      : 1
                   IntraContourFlag                    : 1
                   IntraSdcFlag                        : 1
                   DLT                                 : 1
                   QTL                                 : 0
                   QtPredFlag                          : 0
                   InterSdcFlag                        : 1
                   MpiFlag                             : 0
                   DepthIntraSkip                      : 1

-Picture size:     1920x1088
-Frame rate:       30 frames/s
-Sequence:         Shark
-Num. frames:      30

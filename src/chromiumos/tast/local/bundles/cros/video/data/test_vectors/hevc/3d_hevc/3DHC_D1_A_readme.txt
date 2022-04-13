-Bitstream file:   3DHC_D1_A.hevc
-MD5 file:         3DHC_D1_A.md5
-Specification:    The bitstream is coded with random access configuration as well as 3-view configuration.
                   In addition to inter-view sample prediction for depth, Intra Wedgelet Mode (DMM1) is enabled for depth coding.
-Functional stage: Decoding of three views with Intra Wedgelet Mode enabled for depth views.
-Purpose:          To conform the Intra Wedgelet Mode for depth with random access configuration.
-Software:         HTM-15.1
-Configuration:    (based on random access default configuration)
                   #======== VPS / PTLI ================
                   Profile                             : main main 3d-main
                   Level                               : 5.1  5.1  5.1
                   Tier                                : main main main
                   InblFlag                            : 0    0    0
                   #======== Coding Structure =============
                   #           ...                    interCompPred
                   Frame?_l? : ...                       0
                   #========== multiview coding tools ==========
                   IvMvPredFlag                        : 1 0
                   IvResPredFlag                       : 0
                   IlluCompEnable                      : 0
                   IlluCompLowLatencyEnc               : 0
                   ViewSynthesisPredFlag               : 0
                   DepthRefinementFlag                 : 0
                   IvMvScalingFlag                     : 1
                   Log2SubPbSizeMinus3                 : 0
                   Log2MpiSubPbSizeMinus3              : 0
                   DepthBasedBlkPartFlag               : 0
                   #========== depth coding tools ==========
                   VSO                                 : 1
                   IntraWedgeFlag                      : 1
                   IntraContourFlag                    : 0
                   IntraSdcFlag                        : 0
                   DLT                                 : 0
                   QTL                                 : 0
                   QtPredFlag                          : 0
                   InterSdcFlag                        : 0
                   MpiFlag                             : 0
                   DepthIntraSkip                      : 0

-Picture size:     1920x1088
-Frame rate:       30 frames/s
-Sequence:         Shark
-Num. frames:      30

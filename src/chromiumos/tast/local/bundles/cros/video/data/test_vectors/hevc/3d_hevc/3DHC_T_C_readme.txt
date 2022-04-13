Name: 3DHC_T_C_Sharp
Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to inter-view motion prediction, illumination compensation is enabled in the non-base texture views.
Functional stage: Decoding of three views with illumination compensation.
Purpose: To conform the illumination compensation.
Contributor: Sharp.

Software: HTM-15.1
Configure(cfg file): baseCfg_3view+depth.cfg with interCompPred disabled in RPS + qpCfg_Nview+depth_QP30.cfg + seqCfg_Kendo.cfg
Configure(command line option): -f 64 --Log2SubPbSizeMinus3=3 --IvResPredFlag=0 --IlluCompEnable=1 --ViewSynthesisPredFlag=0\
 --DepthRefinementFlag=0 --DepthBasedBlkPartFlag=0 --QtPredFlag=0 --QTL=0\
 --MpiFlag=0 --Log2MpiSubPbSizeMinus3=3 --IvMvScalingFlag=0\
 --IntraContourFlag=0 --IntraWedgeFlag=0 --IntraSdcFlag=0 --DLT=0 --DepthIntraSkip=0 --InterSdcFlag=0

import subprocess
import sys
import re

class DUT:
  def __init__(self, board, model, hwid, ip, port):
    self.board = board
    self.model = model
    self.hwid = hwid
    self.ip = ip
    self.port = port

class TestIteration:
  def __init__(self, iteratoin_number, state,failure_reason, path):
    self.iteratoin_number = iteratoin_number
    self.state = state
    self.failure_reason = failure_reason
    self.path = path

  def __str__(self):
    return "itr_number:{}, state:{}, path:{}, failure_reason:{}".format(self.iteratoin_number, self.state, self.path, self.failure_reason)

class Test:
  def __init__(self, test_name, args, iterations=10, retries=3):
    self.test_name = test_name
    self.retries = retries
    self.args = args
    self.iterations = iterations
    self.test_iterations = []
  def __str__(self):
    return "name:{}, retries:{}, iterations:{}, itr_count:{}".format(self.test_name, self.retries, self.iterations, len(self.test_iterations))



class Suite:
  def __init__(self, test_list, varsfile):
    self.test_list = test_list
    self.varsfile = varsfile
    self.dry_run = False
    

  def run_suite(self, dut, dry_run=False):
    self.dry_run = dry_run
    for test in self.test_list:
      self.run_test(test, dut)
    return


  def run_test(self, test, dut):
    varfile_tag = "-var=variablesfile={}".format(self.varsfile)
    iteration_tag = "-var=iteration={}".format(test.iterations)
    retry_tag = "-var=retry={}".format(test.retries)
    host_tag = "{}:{}".format(dut.ip, dut.port)

    run_command = ['tast','-verbose', 'run', varfile_tag, " ".join(["-var="+arg for arg in test.args]) , iteration_tag, retry_tag, host_tag, test.test_name]
    
    print("Running command: {}".format(" ".join(run_command)))
    if not self.dry_run:
      process_result = subprocess.run(run_command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
      if process_result.returncode!=0:
        print("Tast failed:{}".format(process_result.stdout.decode("utf-8")))
      print(process_result.stdout.decode('utf-8'))
      self.fetch_test_result_path(process_result.stdout.decode('utf-8'))

  def fetch_test_result_path(self, test_process_output):
    timestamp_re_tag = r"([0-9]+[-|T|:|\.|Z])+ "
    state_expresion = r"{}(.*) \[ (.*) \] (.*)".format(timestamp_re_tag)

    path_patern = r"{}Results saved to (.*)".format(timestamp_re_tag)
    #print(path_patern)
    path_matche = re.search(path_patern, test_process_output) 
    if not path_matche:
      print("Unable to match test path")
      return None
    else:
      #print("Test full path  is {}".format(path_matche.groups(2)[1]))
      path = path_matche.groups(2)[1]
      state_matches = re.finditer(state_expresion, test_process_output)
      if not state_matches:
        print("Unable to find test state for test:{}".format(test.test_name))
      else:
        for _,test_matche in enumerate(state_matches):
          test_name = test_matche.groups(2)[1]
          #print("Matched state_expresion:{}".format(test_name))
          for t in self.test_list:
            if t.test_name == test_name:
              test_iteration = TestIteration(0, test_matche.groups(2)[2], test_matche.groups(2)[3],path + "/tests/"+t.test_name+"/")              
              #print("suite.test:{}, captured_test_iteration:{}".format(t, test_iteration))
              t.test_iterations.append(test_iteration)

        return None
  def pretty_print(self):
    result = ""
    for test in self.test_list:
      test_str = "test_name:{}\n".format(test.test_name)
      for itr in test.test_iterations:
        test_str += str(itr)
      #print("test_str:"+test_str)
      result += test_str
    return result

dut = DUT("dedede", "drawlet","", "localhost","2200")
test_list = []
# test_list.append(Test("ui.GoogleMeetCUJ.basic_two",iterations=1, retries=1, ["ui.cujAccountPool=cros.da.internal1@gmail.com:dvnouavix8", 
#                                                                              "ui.meet_account=cros.da.internal1@gmail.com",
#                                                                              "ui.meet_password=dvnouavix8",
#                                                                              "ui.meet_url_two=https://meet.google.com/xui-hivo-msk"]))

# test_list.append(Test("ui.GoogleMeetCUJ.plus_large"),iterations=1, retries=1, ["ui.cujAccountPool=cros.da.internal1@gmail.com:dvnouavix8", 
#                                                                              "ui.meet_account=cros.da.internal1@gmail.com",
#                                                                              "ui.meet_password=dvnouavix8",
#                                                                              "ui.meet_url_large=https://meet.google.com/cxt-nihe-bsk"])
# test_list.append(Test("ui.GoogleMeetCUJ.premium_large"),iterations=1, retries=1, ["ui.cujAccountPool=cros.da.internal1@gmail.com:dvnouavix8", 
#                                                                              "ui.meet_account=cros.da.internal1@gmail.com",
#                                                                              "ui.meet_password=dvnouavix8",
#                                                                              "ui.meet_url_largehttps://meet.google.com/cxt-nihe-bsk"])
test_list.append(Test("ui.TabSwitchCUJ2.basic_noproxy", [],iterations=1, retries=1))
# test_list.append(Test("ui.QuickCheckCUJ2.basic_unlock",iterations=1, retries=1))
# test_list.append(Test("ui.QuickCheckCUJ2.basic_wakeup",iterations=1, retries=1))
# test_list.append(Test("ui.VideoCUJ2.basic_youtube_web",iterations=1, retries=1))
# test_list.append(Test("ui.EverydayMultiTaskingCUJ.basic_ytmusic",[], iterations=1, retries=1))


input="""
rpc error: code = Unknown desc = failed to run Google Meet conference: failed to conduct the recorder task: failed to join conference on step 4: failed to switch account: failed to switch account on step 1: failed to initially click the node: context deadline exceeded; last error follows: Uncaught (in promise): "failed to find node with properties: {name: /^Switch account$/, role: link}"
2022-03-14T02:41:34.133622Z [02:41:34.133] Connecting to browser at ws://localhost:39925/devtools/browser/3be77dcd-ecb1-476b-b15f-7f852dcc1009
2022-03-14T02:41:34.133696Z [02:41:34.133] Connecting to Chrome target 996F0C1A8F2A8C7261B1A8941914F752
2022-03-14T02:41:34.133711Z [02:41:34.133] Taking screenshot via chrome API
2022-03-14T02:41:36.307578Z Completed test ui.GoogleMeetCUJ.basic_two in 1m13.386s with 1 error(s)
2022-03-14T02:41:36.312895Z Got global error: unknown event type <nil>
2022-03-14T02:41:36.655920Z Collecting system information
2022-03-14T02:41:55.690061Z --------------------------------------------------------------------------------
2022-03-14T02:41:55.690107Z ui.GoogleMeetCUJ.basic_two [ FAIL ] Failed to run Meet Scenario: rpc error: code = Unknown desc = failed to run Google Meet conference: failed to conduct the recorder task: failed to join conference on step 4: failed to switch account: failed to switch account on step 1: failed to initially click the node: context deadline exceeded; last error follows: Uncaught (in promise): "failed to find node with properties: {name: /^Switch account$/, role: link}"
2022-03-14T02:41:55.690118Z --------------------------------------------------------------------------------
2022-03-14T02:41:55.690124Z Results saved to /tmp/tast/results/20220314-023959
"""


quick_suite = Suite(test_list, "meta.yaml")
quick_suite.run_suite(dut)
#quick_suite.fetch_test_result_path(input)
print("pretty print:"+quick_suite.pretty_print())

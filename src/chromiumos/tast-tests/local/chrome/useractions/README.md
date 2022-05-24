# UserContext & UserAction

In a test built from user journey, recording the DUT environment information can
satisfy more complicated test analysis requirement.

## UserContext

UserContext represents the environment along with the test running. It can
either be inherited from precondition / fixture, or newly created in the middle
of a test script.

UserContext normally contains the platform & system information, such as `Board
name`, `Device mode`, `Keyboard type`.

Note: Do not frequently update userContext during a test, which can mess up the
data or inconsistency between data and real physical device information.

## UserAction

UserAction represents one or more actions performed by the end user. The result
of the action is automatically collected on the interest of its creator.

UserAction is not necessary to be a regular test case, it is literally an
uiauto.Action with more attached information.

UserAction borrows the idea from Javascript Promise, providing 3 optional
functions (ifSuccessFunc,ifFailFunc, finalFunc) to follow up the actual action.
They are designed to update UserContext, cleanup environment, etc. Any errors
occurring in these functions are informational logged only.

whilst UserContext represents the DUT environment that the action is running in.

## Run and Log

Once an UserAction runs by any trigger of below,the action result is
automatically logged into `user_action_log.csv` file under the test output
directory `s.OutDir()`.

*   (uc *UserContext) RunAction
*   (ua *UserAction) Run

## Attributes and Tags

Both UserContext and UserAction have attributes and tags. Attributes are saved
in map to accommodate key values (e.g. KeyboardType: Tablet VK), while Tags uses
list to represent key information of the action (e.g. relevant CUJ Names:
VKTyping, ARC++ ). Once logging action results, context attributes and tags are
merged into action attributes and tags.

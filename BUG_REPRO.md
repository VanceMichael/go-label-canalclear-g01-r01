# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

闸口状态变更时会同时通知海关和船代。操作员取消请求后，接口已经结束，邮件发送器却继续占着连接，第二条通知甚至在取消后才开始；遇到消息服务变慢时，后台任务会拖到内部超时才释放。请修复这条取消链路，让正在发送和尚未调度的通知都及时停止，结果仍按原投递顺序完整返回，正常通知不受影响。

## 含 Bug 版本

- 仓库：VanceMichael/go-label-canalclear-g01-r01
- 仓库地址：https://github.com/VanceMichael/go-label-canalclear-g01-r01.git
- parent SHA：8e9115cd7e205ee8509232a9b79f3d1bc0960557

## 复现步骤

```bash
git clone -- https://github.com/VanceMichael/go-label-canalclear-g01-r01.git bug-repro
cd bug-repro
git checkout --detach 8e9115cd7e205ee8509232a9b79f3d1bc0960557
go test ./internal/notification -run ^TestCancelledDispatchStopsOutstandingPortNotifications$ -count=1
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/notification -run ^TestCancelledDispatchStopsOutstandingPortNotifications$ -count=1
--- FAIL: TestCancelledDispatchStopsOutstandingPortNotifications (0.31s)
    cancellation_contract_test.go:59: dispatch did not return after caller cancellation
FAIL
FAIL	github.com/VanceMichael/go-base-canalclear-g01/internal/notification	0.338s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/notification -run ^TestCancelledDispatchStopsOutstandingPortNotifications$ -count=1
--- FAIL: TestCancelledDispatchStopsOutstandingPortNotifications (0.30s)
    cancellation_contract_test.go:59: dispatch did not return after caller cancellation
FAIL
FAIL	github.com/VanceMichael/go-base-canalclear-g01/internal/notification	0.303s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

调用方取消闸口通知批次后，已进入 Sender.Send 的操作必须收到 context.Canceled，尚未开始的投递不得再调用发送器，并在 300ms 内返回与输入等长、顺序一致的失败结果；未取消的批次仍按原渠道正常送达。go test ./internal/notification -run ^TestCancelledDispatchStopsOutstandingPortNotifications$ -count=1 应从超时失败变为通过，notification 包回归、全仓 go test ./... 和 go build ./... 均须正常；不得缩短测试等待、用后台 context 继续发送或删去未调度结果。

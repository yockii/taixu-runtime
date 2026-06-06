package egress

import "sync"

// 测试钩子：复位包级单例状态，让多用例间互不污染。

func resetForTest() {
	reset()
	subOnce = sync.Once{}
	resolver = defaultPeerResolver
}

func setResolverForTest(r peerResolver) { resolver = r }

// 暴露内部路由函数供单测直接断言（不经 bus 订阅）。
var (
	dispatchSendForTest     = dispatchSend
	dispatchApprovalForTest = dispatchApproval
	resolveTargetForTest    = resolveTarget
)

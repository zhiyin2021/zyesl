package kernel

type FSound string

const (
	// 对不起,您所呼叫的用户忙,请稍后再拨
	UserBusy = FSound("user_busy")
	// 对不起,你所呼叫的用户是空号,请稍后再拨
	UserNotFound = FSound("user_not_found")
	// 对不起,您所呼叫的用户暂时无法接通,请稍后再拨
	TemporarilyUnavailable = FSound("temporarily_unavailable")
	// 对不起,您所呼叫的用户线故障,请稍后再拨
	UserLineFault = FSound("user_line_fault")
	// 签到成功
	SigninOk = FSound("signin_ok")
	// 接入音
	IncomingTip = FSound("incoming_tip")
	// 客户已接入,请注意接听
	// CustIncoming = FSound("cust_incoming")
	// 等待音
	Ringback = FSound("ringback")
	// 正在为你转接客服,请稍候
	QueueRingBack = FSound("queue_ringback")
	// 正在接听,请稍候
	Connecting = FSound("connecting")
	// 整理等待音
	CallOver = FSound("call_over")
)

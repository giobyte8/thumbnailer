package messaging

type MsgProcessor interface {
	onMessage(msgBody []byte) error
}

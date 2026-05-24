package consumer

type MsgProcessor interface {
	onMessage(msgBody []byte) error
}

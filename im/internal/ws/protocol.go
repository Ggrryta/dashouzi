package ws

type Message struct {
	Type    string `json:"type"`    // msg / ack / ping / pong
	From    uint   `json:"from"`
	To      uint   `json:"to"`
	Content string `json:"content"`
	MsgID   string `json:"msg_id"`
}

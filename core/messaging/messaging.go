package messaging

const (
	startType = "start"
)

var Messages = map[string]Message{
	startType: StartMessage{},
}

type Message interface {
	Empty() Message
	Type() string
}

type StartMessage struct{}

func (StartMessage) Empty() Message {
	return &StartMessage{}
}

func (StartMessage) Type() string {
	return startType
}

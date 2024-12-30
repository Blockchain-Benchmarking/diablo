package messaging

const (
	PrimaryInitType   = "pinit"
	SecondaryInitType = "sinit"

	StartType   = "start"
	StopType    = "stop"
	StoppedType = "stopped"

	UsersType   = "users"
	ResultsType = "results"
)

var Messages = map[string]Message{
	PrimaryInitType:   PrimaryInit{},
	SecondaryInitType: SecondaryInit{},

	StartType:   Start{},
	StopType:    Stop{},
	StoppedType: Stopped{},

	UsersType:   Users{},
	ResultsType: Results{},
}

type Message interface {
	Empty() Message
	Type() string
}

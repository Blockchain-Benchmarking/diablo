package messaging

const (
	PrimaryInitType   = "pinit"
	SecondaryInitType = "sinit"

	StartType   = "start"
	StopType    = "stop"
	StoppedType = "stopped"

	UsersType   = "users"
	ResultsType = "results"
	//StatusType   = "status"
)

var Messages = map[string]Message{
	PrimaryInitType:   PrimaryInit{},
	SecondaryInitType: SecondaryInit{},

	StartType:   Start{},
	StopType:    Stop{},
	StoppedType: Stopped{},

	UsersType:   Users{},
	ResultsType: Results{},
	//StatusType:   Status{},
}

type Message interface {
	Empty() Message
	Type() string
}

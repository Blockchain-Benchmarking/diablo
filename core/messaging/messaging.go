package messaging

const (
	PrimaryInitType   = "pinit"
	SecondaryInitType = "sinit"

	StartType = "start"

	WorkloadType = "workload"
	ResultsType  = "results"
	StatusType   = "status"
)

var Messages = map[string]Message{
	PrimaryInitType:   PrimaryInit{},
	SecondaryInitType: SecondaryInit{},

	StartType: Start{},

	WorkloadType: Workload{},
	ResultsType:  Results{},
	StatusType:   Status{},
}

type Message interface {
	Empty() Message
	Type() string
}

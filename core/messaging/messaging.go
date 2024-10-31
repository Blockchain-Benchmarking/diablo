package messaging

const (
	PrimaryInitType   = "pinit"
	SecondaryInitType = "sinit"

	WorkloadType = "workload"
	MoreType     = "more"
	StopType     = "stop"
	ResultsType  = "results"
)

var Messages = map[string]Message{
	PrimaryInitType:   PrimaryInitMessage{},
	SecondaryInitType: SecondaryInitMessage{},

	WorkloadType: Workload{},
	MoreType:     More{},
	StopType:     Stop{},
	ResultsType:  Results{},
}

type Message interface {
	Empty() Message
	Type() string
}

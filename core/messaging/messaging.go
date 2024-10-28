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

// PrimaryInitMessage indicates which generator should be used by the secondary
type PrimaryInitMessage struct {
	Workload string `json:"workload"`
}

func (PrimaryInitMessage) Empty() Message {
	return &PrimaryInitMessage{}
}

func (PrimaryInitMessage) Type() string {
	return PrimaryInitType
}

// SecondaryInitMessage indicates that secondary's generator is ready to handle workload messages
type SecondaryInitMessage struct {
	Tags []string `json:"tags"`
}

func (SecondaryInitMessage) Empty() Message {
	return &SecondaryInitMessage{}
}

func (SecondaryInitMessage) Type() string {
	return SecondaryInitType
}

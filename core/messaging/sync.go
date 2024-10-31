package messaging

// PrimaryInitMessage indicates which generator should be used by the secondary
type PrimaryInitMessage struct {
	Duration string `json:"duration"`
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

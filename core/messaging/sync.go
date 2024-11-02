package messaging

// PrimaryInit indicates which generator should be used by the secondary
type PrimaryInit struct {
	Duration string `json:"duration"`
}

func (PrimaryInit) Empty() Message {
	return &PrimaryInit{}
}

func (PrimaryInit) Type() string {
	return PrimaryInitType
}

// SecondaryInit indicates that secondary's generator is ready to handle workload messages
type SecondaryInit struct {
	Tags []string `json:"tags"`
}

func (SecondaryInit) Empty() Message {
	return &SecondaryInit{}
}

func (SecondaryInit) Type() string {
	return SecondaryInitType
}

type Start struct {
	Start int64 `json:"start"`
}

func (Start) Empty() Message {
	return &Start{}
}

func (Start) Type() string {
	return StartType
}

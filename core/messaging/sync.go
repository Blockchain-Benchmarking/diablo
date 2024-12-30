package messaging

// PrimaryInit indicates the duration of the experiment
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

// Start indicates the starting time of the experiment
type Start struct {
	Start int64 `json:"start"`
}

func (Start) Empty() Message {
	return &Start{}
}

func (Start) Type() string {
	return StartType
}

// Stop indicates to the secondary that it should stop the experiment and that no new users will be sent
type Stop struct{}

func (Stop) Empty() Message {
	return &Stop{}
}

func (Stop) Type() string {
	return StopType
}

// Stopped indicates to the primary that the secondary is done
type Stopped struct{}

func (Stopped) Empty() Message {
	return &Stopped{}
}

func (Stopped) Type() string {
	return StoppedType
}

package messaging

type Workload struct {
	Users []byte `json:"users"`
	Done  bool   `json:"done"`
}

func (Workload) Empty() Message {
	return &Workload{}
}

func (Workload) Type() string {
	return WorkloadType
}

type More struct {
	Source string `json:"source"`
}

func (More) Empty() Message {
	return &More{}
}

func (More) Type() string {
	return MoreType
}

type Stop struct{}

func (Stop) Empty() Message {
	return &Stop{}
}

func (Stop) Type() string {
	return StopType
}

type Results struct {
	Results []byte `json:"results"`
}

func (Results) Empty() Message {
	return &Results{}
}

func (Results) Type() string {
	return ResultsType
}

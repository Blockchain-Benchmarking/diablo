package messaging

type Workload struct {
	Users []byte `json:"users"`
}

func (Workload) Empty() Message {
	return &Workload{}
}

func (Workload) Type() string {
	return WorkloadType
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

type Status struct {
	Timestamp    int64 `json:"timestamp"`
	CpuUsage     int   `json:"cpu_usage"`
	MemUsage     int   `json:"mem_usage"`
	UsersRunning int   `json:"users_running"`
}

func (Status) Empty() Message {
	return &Status{}
}

func (Status) Type() string {
	return StatusType
}

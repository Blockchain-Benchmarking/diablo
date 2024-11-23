package messaging

type Users struct {
	Users map[string][]byte `json:"users"` //userType -> users
}

func (Users) Empty() Message {
	return &Users{}
}

func (Users) Type() string {
	return UsersType
}

type Results struct {
	Name    string `json:"name"`
	Results []byte `json:"results"`
}

func (Results) Empty() Message {
	return &Results{}
}

func (Results) Type() string {
	return ResultsType
}

/**
type Status struct {
	Timestamp    int64 `json:"timestamp"`
	CpuUsage     int   `json:"cpu_usage"`
	MemUsage     int   `json:"mem_usage"`
	UsersRunning int   `json:"users_running"`
	//transactions fail
}

func (Status) Empty() Message {
	return &Status{}
}

func (Status) Type() string {
	return StatusType
}*/

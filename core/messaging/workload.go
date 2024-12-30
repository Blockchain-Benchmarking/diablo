package messaging

// Users contains a map associating a user type to a list of marshalled users
type Users struct {
	Users map[string][]byte `json:"users"`
}

func (Users) Empty() Message {
	return &Users{}
}

func (Users) Type() string {
	return UsersType
}

// Results contains a result type and a list of marshalled results
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

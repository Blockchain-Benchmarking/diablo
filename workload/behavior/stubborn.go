package behavior

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const StubbornResult = "stubborn-result"

var TimeoutError = errors.New("timeout")

type StubbornBehavior struct {
	MaxAttempts int32 `json:"max_attempts"`
}

func NewStubbornBehavior(maxRetries int32) *StubbornBehavior {
	return &StubbornBehavior{maxRetries}
}

func (s *StubbornBehavior) PerformStubbornAction(f func() error, actionType string) StubbornAction {
	action := StubbornAction{
		Type: actionType,
	}
	action.StartTime = time.Now()
	action.Attempts = 1

	var err error
	for i := 0; i < int(s.MaxAttempts); i++ {
		err = f()
		if err == nil {
			action.Passed = true
			action.EndTime = time.Now()
			return action
		} else {
			if errors.Is(TimeoutError, err) {
				action.Timeout = true
			}
		}

		action.Attempts++
	}

	action.EndTime = time.Now()
	return action
}

type StubbornAction struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Type      string    `json:"type"`
	Attempts  int       `json:"attempts"`
	Passed    bool      `json:"success"`
	Timeout   bool      `json:"timeout"`
}

func (s StubbornAction) Start() time.Time {
	return s.StartTime
}

func (s StubbornAction) End() time.Time {
	return s.EndTime
}

func (s StubbornAction) Success() bool {
	return s.Passed
}

func (s StubbornAction) Type() string {
	return StubbornResult
}

func (s StubbornAction) Empty() Result {
	return &StubbornAction{}
}

func (s StubbornAction) UnmarshalResults(buf []byte) ([]Result, error) {
	var res []*StubbornAction
	err := json.Unmarshal(buf, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal stubborn payment users: %w", err)
	}

	results := make([]Result, len(res))
	for i, r := range res {
		results[i] = r
	}

	return results, nil
}

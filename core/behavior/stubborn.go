package behavior

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type StubbornBehavior struct {
	MaxRetries int32
}

type StubbornAction struct {
	StartTime        int64 // 0 if not submitted
	SuccessTime      int64 // 0 if not committed
	FinalFailureTime int64 // 0 if not aborted
	Retries          int32
	HasError         bool
}

// type StubbornResult []SingleUserStubbornResult
type SingleUserStubbornResult []*StubbornAction

func (s *SingleUserStubbornResult) Decode(src *bufio.Reader) error {
	var userResultLength int32
	if err := binary.Read(src, binary.LittleEndian, &userResultLength); err != nil {
		return fmt.Errorf("failed to read user result length at %d: %w", userResultLength, err)
	}

	for i := int32(0); i < userResultLength; i++ {
		action := &StubbornAction{}

		if err := binary.Read(src, binary.LittleEndian, &action.StartTime); err != nil {
			return fmt.Errorf("failed to read submit time at %d: %w", i, err)
		}
		if err := binary.Read(src, binary.LittleEndian, &action.SuccessTime); err != nil {
			return fmt.Errorf("failed to read commit time at %d: %w", i, err)
		}
		if err := binary.Read(src, binary.LittleEndian, &action.FinalFailureTime); err != nil {
			return fmt.Errorf("failed to read abort time at %d: %w", i, err)
		}
		if err := binary.Read(src, binary.LittleEndian, &action.Retries); err != nil {
			return fmt.Errorf("failed to read retries at %d: %w", i, err)
		}
		if err := binary.Read(src, binary.LittleEndian, &action.HasError); err != nil {
			return fmt.Errorf("failed to read has error at %d: %w", i, err)
		}

		*s = append(*s, action)
	}

	return nil
}

func (s *SingleUserStubbornResult) Encode() ([]byte, error) {
	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.LittleEndian, int32(len(*s))); err != nil {
		return nil, err
	}

	for _, action := range *s {
		if err := binary.Write(&buf, binary.LittleEndian, action.StartTime); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, action.SuccessTime); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, action.FinalFailureTime); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, action.Retries); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, action.HasError); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (s *SingleUserStubbornResult) Merge(other Results) Results {
	otherSimpleResult, ok := other.(*SingleUserStubbornResult)
	if !ok {
		panic("tried to merge different result implementations")
	}

	res := &SingleUserStubbornResult{}
	*res = append(*s, *otherSimpleResult...)

	return res
}

func (s *SingleUserStubbornResult) PrintResult(dest io.Writer) error {
	buf, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	_, err = dest.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

func NewStubbornBehavior(maxRetries int32) *StubbornBehavior {
	return &StubbornBehavior{maxRetries}
}

func (s *StubbornBehavior) PerformStubbornAction(f func() error) *StubbornAction {
	action := &StubbornAction{}
	action.StartTime = time.Now().Unix()

	var err error
	for i := 0; i < int(s.MaxRetries); i++ {
		err = f()
		if err == nil {
			action.SuccessTime = time.Now().Unix()
			return action
		}

		action.Retries++
	}

	action.FinalFailureTime = time.Now().Unix()
	action.HasError = true

	return action
}

func (s *StubbornBehavior) Encode(dest io.Writer) error {
	err := binary.Write(dest, binary.LittleEndian, int32(s.MaxRetries))
	if err != nil {
		return fmt.Errorf("failed to encode max retries")
	}

	return nil
}

func (s *StubbornBehavior) Decode(src *bufio.Reader) error {
	err := binary.Read(src, binary.LittleEndian, &s.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to decode max retries")
	}

	return nil
}

/**
func (s *StubbornResult) Encode() ([]byte, error) {
	var buf bytes.Buffer

	logging.Infof("encoding %d results", len(*s))
	if err := binary.Write(&buf, binary.LittleEndian, int32(len(*s))); err != nil {
		return nil, err
	}

	for _, userResult := range *s {
		tmp, err := userResult.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode user result: %w", err)
		}

		err = binary.Write(&buf, binary.LittleEndian, tmp)
		if err != nil {
			return nil, fmt.Errorf("failed to write user result: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// Decode implements Results
func (s *StubbornResult) Decode(src *bufio.Reader) error {
	var resultLength int32
	if err := binary.Read(src, binary.LittleEndian, &resultLength); err != nil {
		return fmt.Errorf("failed to read result length: %w", err)
	}

	*s = make(StubbornResult, resultLength)

	for i := int32(0); i < resultLength; i++ {
		var userResultLength int32
		if err := binary.Read(src, binary.LittleEndian, &userResultLength); err != nil {
			return fmt.Errorf("failed to read user result length at %d: %w", i, err)
		}

		logging.Debugf("user result length: %d", userResultLength)
		userResult := make(SingleUserStubbornResult, userResultLength)

		for j := int32(0); j < userResultLength; j++ {
			action := &StubbornAction{}

			if err := binary.Read(src, binary.LittleEndian, &action.StartTime); err != nil {
				return fmt.Errorf("failed to read submit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.SuccessTime); err != nil {
				return fmt.Errorf("failed to read commit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.FinalFailureTime); err != nil {
				return fmt.Errorf("failed to read abort time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.Retries); err != nil {
				return fmt.Errorf("failed to read retries at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.HasError); err != nil {
				return fmt.Errorf("failed to read has error at %d, %d: %w", i, j, err)
			}

			userResult[j] = action
		}

		(*s)[i] = userResult
	}

	return nil
}

// Merge implements Results
func (s *StubbornResult) Merge(other Results) Results {
	switch otherTyped := other.(type) {
	case *StubbornResult:
		mergedResult := append(*s, *otherTyped...)
		return &mergedResult
	case *SingleUserStubbornResult:
		// Handle SimpleResult merging if needed
		mergedResult := append(*s, *otherTyped)
		return &mergedResult
	default:
		panic(fmt.Sprintf("tried to merge incompatible result types: %T", other))
	}
}

// PrintResult implements Results
func (s *StubbornResult) PrintResult(dest io.Writer) error {
	buf, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	_, err = dest.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}*/

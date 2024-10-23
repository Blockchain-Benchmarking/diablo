package behavior

import (
	"diablo/core/logging"
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

type StubbornResult []SingleUserStubbornResult
type SingleUserStubbornResult []*StubbornAction

func (s SingleUserStubbornResult) Encode(dest io.Writer) error {
	panic("encoding and decoding aren't implemented for SingleUserStubbornResult")
}

func (s SingleUserStubbornResult) Decode(src io.Reader) error {
	panic("encoding and decoding aren't implemented for SingleUserStubbornResult")
}

func (s SingleUserStubbornResult) Merge(other Results) Results {
	otherSimpleResult, ok := other.(SingleUserStubbornResult)
	if !ok {
		panic("tried to merge different result implementations")
	}

	return &StubbornResult{s, otherSimpleResult}
}

func (s SingleUserStubbornResult) PrintResult(dest io.Writer) error {
	//TODO implement me
	panic("implement me")
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

func (s *StubbornBehavior) Decode(src io.Reader) error {
	err := binary.Read(src, binary.LittleEndian, &s.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to decode max retries")
	}

	return nil
}

// Encode implements Results
func (s *StubbornResult) Encode(dest io.Writer) error {
	logging.Infof("writing length %d", int32(len(*s)))
	if err := binary.Write(dest, binary.LittleEndian, int32(len(*s))); err != nil {
		return err
	}

	for _, userResult := range *s {
		if err := binary.Write(dest, binary.LittleEndian, int32(len(userResult))); err != nil {
			return err
		}

		for _, action := range userResult {
			if err := binary.Write(dest, binary.LittleEndian, action.StartTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.SuccessTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.FinalFailureTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.Retries); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.HasError); err != nil {
				return err
			}
		}
	}
	return nil
}

// Decode implements Results
func (s *StubbornResult) Decode(src io.Reader) error {
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
	case SingleUserStubbornResult:
		// Handle SimpleResult merging if needed
		mergedResult := append(*s, otherTyped)
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
}

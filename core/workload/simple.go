package workload

import (
	"diablo/core/logging"
	"diablo/core/user"
	"encoding/binary"
	"fmt"
	"io"
)

type UserInfoWorkload struct {
	UserInfos []user.UserInfo
}

func (w *UserInfoWorkload) Encode(dest io.Writer) error {
	count := int32(len(w.UserInfos))
	err := binary.Write(dest, binary.LittleEndian, count)
	if err != nil {
		return fmt.Errorf("failed to encode UserInfoWorkload count %d: %w", count, err)
	}

	for _, userInfo := range w.UserInfos {
		err = userInfo.Encode(dest)
		if err != nil {
			return fmt.Errorf("failed to encode userInfo: %w", err)
		}
	}

	return nil
}

func (w *UserInfoWorkload) Decode(src io.Reader) error {
	logging.Debugf("decoding user info workload")
	var count int32
	err := binary.Read(src, binary.LittleEndian, &count)
	if err != nil {
		return err
	}

	w.UserInfos = make([]user.UserInfo, count)

	for i := int32(0); i < count; i++ {
		logging.Debugf("decoding user info")
		userInfo := &user.UserInfo{}
		err = userInfo.Decode(src)
		if err != nil {
			return err
		}
		logging.Debugf("decodied user info with pk len %d", len(userInfo.Account.PrivateKey))
		w.UserInfos[i] = *userInfo
	}

	return nil
}

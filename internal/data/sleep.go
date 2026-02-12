package data

import (
	"encoding/json"
)

type Sleep struct {
	Id       string `json:"sleep_id"`
	Duration int64  `json:"sleep_duration,string"`
}

func (s *Sleep) MarshalBinary() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Sleep) UnmarshalBinary(bytes []byte) error {
	return json.Unmarshal(bytes, s)
}

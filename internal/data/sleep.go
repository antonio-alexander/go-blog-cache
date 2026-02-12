package data

import (
	"encoding/json"
	"time"
)

type Sleep struct {
	Id       string        `json:"sleep_id"`
	Duration time.Duration `json:"sleep_duration"`
}

func (s *Sleep) MarshalBinary() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Sleep) UnmarshalBinary(bytes []byte) error {
	return json.Unmarshal(bytes, s)
}

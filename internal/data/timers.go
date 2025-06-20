package data

type Timers struct {
	Totals   map[string]int64 `json:"totals,omitempty"`
	Averages map[string]int64 `json:"averages,omitempty"`
}

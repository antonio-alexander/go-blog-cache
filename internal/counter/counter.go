package counter

type Count struct {
	Hit  int `json:"hit"`
	Miss int `json:"miss"`
}

type Counter struct {
	Counts map[string]*Count
}

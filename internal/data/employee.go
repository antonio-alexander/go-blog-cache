package data

import "encoding/json"

type Employee struct {
	EmpNo     int64  `json:"emp_no"`
	BirthDate int64  `json:"birth_date"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Gender    string `json:"gender"`
	HireDate  int64  `json:"hire_date"`
}

func (e *Employee) MarshalBinary() ([]byte, error) {
	return json.Marshal(e)
}

func (e *Employee) UnmarshalBinary(bytes []byte) error {
	return json.Unmarshal(bytes, e)
}

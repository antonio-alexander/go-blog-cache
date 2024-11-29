package data

import "encoding/json"

type Employee struct {
	EmpNo     int64  `json:"emp_no"` //this is actually an int32, but the types are compatible
	BirthDate int64  `json:"birth_date"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Gender    string `json:"gender"` //this is actually an enum, but not worth the effort
	HireDate  int64  `json:"hire_date"`
}

func (e *Employee) MarshalBinary() ([]byte, error) {
	return json.Marshal(e)
}

func (e *Employee) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, e)
}

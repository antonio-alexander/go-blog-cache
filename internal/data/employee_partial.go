package data

import "encoding/json"

type EmployeePartial struct {
	BirthDate *int64  `json:"birth_date,omitempty,string"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Gender    *string `json:"gender,omitempty"` //this is actually an enum, but not worth the effort
	HireDate  *int64  `json:"hire_date,omitempty,string"`
}

func (e *EmployeePartial) MarshalBinary() ([]byte, error) {
	return json.Marshal(e)
}

func (e *EmployeePartial) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, e)
}

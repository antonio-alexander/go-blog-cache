package data

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type EmployeeSearch struct {
	EmpNos     []int64  `json:"emp_nos"`
	FirstNames []string `json:"first_names"`
	LastNames  []string `json:"last_names"`
	Gender     string   `json:"gender"`
}

func (e *EmployeeSearch) ToParams() url.Values {
	params := make(map[string][]string)
	if len(e.EmpNos) > 0 {
		var empNos []string
		for _, empNo := range e.EmpNos {
			empNos = append(empNos, fmt.Sprint(empNo))
		}
		params[ParameterEmpNos] = append(params[ParameterEmpNos], strings.Join(empNos, ","))
	}
	return params
}

func (e *EmployeeSearch) FromParams(params url.Values) {
	for key, value := range params {
		switch strings.ToLower(key) {
		case ParameterEmpNos:
			for _, value := range value {
				for _, v := range strings.Split(value, ",") {
					empNo, _ := strconv.ParseInt(v, 10, 64)
					e.EmpNos = append(e.EmpNos, empNo)
				}
			}
		}
	}
}

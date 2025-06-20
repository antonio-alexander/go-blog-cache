package data

const (
	RouteEmployees       string = "/employees"
	RouteEmployeesSearch string = RouteEmployees + "/search"
	RouteEmployeesEmpNo  string = RouteEmployees + "/{" + PathEmpNo + "}"
	RouteEmployeesEmpNof string = RouteEmployees + "/%d"
	RouteTimers          string = "/timers"
	RouteCacheCounters   string = "/cachecounters"
)

const PathEmpNo string = "EmpNo"

const ParameterEmpNos string = "emp_nos"

type Request struct {
	EmployeePartial EmployeePartial `json:"employee_partial"`
}

type Response struct {
	Employee  *Employee   `json:"employee,omitempty"`
	Employees []*Employee `json:"employees,omitempty"`
}

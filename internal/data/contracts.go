package data

const (
	RouteEmployees       string = "/employees"
	RouteEmployeesSearch string = RouteEmployees + "/search"
	RouteEmployeesNo     string = RouteEmployees + "/{" + PathEmpNo + "}"
	RouteEmployeesNof    string = RouteEmployees + "/%d"
)

const PathEmpNo string = "EmpNo"

const ParameterEmpNos string = "emp_nos"

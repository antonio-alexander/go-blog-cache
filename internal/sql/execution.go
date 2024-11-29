package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

func employeeCriteria(search data.EmployeeSearch) (string, []interface{}) {
	var args []interface{}
	var criteria []string

	if empNos := search.EmpNos; len(empNos) > 0 {
		var parameters []string

		for _, empNo := range empNos {
			args = append(args, empNo)
			parameters = append(parameters, "?")
		}
		criteria = append(criteria, fmt.Sprintf("emp_no IN(%s)", strings.Join(parameters, ",")))
	}
	if len(criteria) <= 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(criteria, " AND "), args
}

func employeeScan(scanFx func(...interface{}) error) (*data.Employee, error) {
	var hireDate, birthDate time.Time

	employee := new(data.Employee)
	if err := scanFx(
		&employee.EmpNo,
		&birthDate,
		&employee.FirstName,
		&employee.LastName,
		&employee.Gender,
		&hireDate,
	); err != nil {
		return nil, err
	}
	employee.BirthDate = birthDate.Unix()
	employee.HireDate = hireDate.Unix()
	return employee, nil
}

func findEmpNo(ctx context.Context, db *sql.DB) (int64, error) {
	var empNo int64

	query := fmt.Sprintf("SELECT emp_no FROM %s ORDER BY emp_no DESC LIMIT 1;",
		tableEmployees)
	row := db.QueryRowContext(ctx, query)
	if err := row.Scan(&empNo); err != nil {
		return -1, err
	}
	return empNo + 1, nil
}

package sql

import (
	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

var (
	ErrEmployeeNotFound       = data.NewNotFoundError("employee not found")
	ErrEmployeeSearchNotFound = data.NewNotFoundError("employee search not found")
)

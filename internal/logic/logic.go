package logic

import (
	"sync"

	"github.com/antonio-alexander/go-blog-cache/internal/sql"
)

type Logic struct {
	sync.RWMutex
	*sql.Sql
}

func NewLogic(parameters ...interface{}) *Logic {
	l := &Logic{}
	for _, parameter := range parameters {
		switch v := parameter.(type) {
		case *sql.Sql:
			l.Sql = v
		}
	}
	return l
}

func (l *Logic) Configure(envs map[string]string) error {
	return nil
}

func (l *Logic) Open() error {
	return nil
}

func (l *Logic) Close() error {
	return nil
}

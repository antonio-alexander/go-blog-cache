package internal

import "context"

type Configurer interface {
	Configure(envs map[string]string) error
}

type Opener interface {
	Open(ctx context.Context) error
	Closer
}

type Closer interface {
	Close(ctx context.Context) error
}

type Clearer interface {
	Clear(ctx context.Context) error
}

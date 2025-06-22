package sources

import "context"

type Source interface {
	Start() error
	Stop(ctx context.Context) error
}

package consumer

import (
	"context"
)

type MessageConsumer interface {
	Start(ctx context.Context) error

	Stop()
}

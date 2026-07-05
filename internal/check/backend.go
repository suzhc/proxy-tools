package check

import (
	"context"

	"github.com/suzhc/px-health/internal/link"
)

type Backend interface {
	Supports(node link.Node) bool
	Start(ctx context.Context, node link.Node) (LocalProxy, error)
}

type LocalProxy struct {
	SOCKSAddress string
	Cleanup      func()
}

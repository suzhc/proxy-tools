package check

import (
	"context"

	"github.com/suzhc/proxy-tools/internal/link"
)

type Backend interface {
	Supports(node link.Node) bool
	Start(ctx context.Context, node link.Node) (LocalProxy, error)
}

type LocalProxy struct {
	SOCKSAddress string
	Cleanup      func()
}

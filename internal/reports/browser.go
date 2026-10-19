package reports

import (
	"context"
	"github.com/shesh/gradia/internal/desktop"
)

func OpenInBrowser(ctx context.Context, path string) error {
	return desktop.OpenFile(ctx, path)
}

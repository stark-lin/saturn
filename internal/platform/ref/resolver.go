// This file resolves reference codes to registered object references.
package ref

import (
	"context"
	"errors"
)

var (
	ErrInvalidCode = errors.New("invalid ref code")
	ErrNotFound    = errors.New("object ref not found")
)

type Resolver struct {
	repo Repository
}

func NewResolver(repo Repository) *Resolver {
	return &Resolver{repo: repo}
}

func (r *Resolver) Resolve(ctx context.Context, code string) (ObjectRef, error) {
	code = NormalizeCode(code)
	if !ValidCode(code) {
		return ObjectRef{}, ErrInvalidCode
	}
	return r.repo.FindByCode(ctx, code)
}

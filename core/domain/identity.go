package domain

import "slices"

type Identity struct {
	UserID string
	Scopes []string
}

func (i *Identity) HasScope(scope string) bool {
	return slices.Contains(i.Scopes, scope)
}

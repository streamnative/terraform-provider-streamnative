package cloud

import (
	"errors"

	"github.com/streamnative/cloud-cli/pkg/auth"
)

type MemoryStore struct {
	data map[string]*data
}

type data struct {
	username string
	grant    auth.AuthorizationGrant
}

// ErrNoAuthenticationData indicates that stored authentication data is not available
var ErrNoAuthenticationData = errors.New("authentication data is not available")

// ErrUnsupportedAuthData ndicates that stored authentication data is unusable
var ErrUnsupportedAuthData = errors.New("authentication data is not usable")

// Store is responsible for persisting authorization grants
type Store interface {
	// SaveGrant stores an authorization grant for a given audience
	SaveGrant(audience string, grant auth.AuthorizationGrant) error

	// LoadGrant loads an authorization grant for a given audience
	LoadGrant(audience string) (*auth.AuthorizationGrant, error)

	// WhoAmI returns the current user name (or an error if nobody is logged in)
	WhoAmI(audience string) (string, error)

	// Logout deletes all stored credentials
	Logout() error
}

var _ Store = &MemoryStore{}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]*data),
	}
}

func (s *MemoryStore) SaveGrant(audience string, grant auth.AuthorizationGrant) error {
	s.data[audience] = &data{
		username: grant.ClientCredentials.ClientEmail,
		grant:    grant,
	}
	return nil
}

func (f *MemoryStore) LoadGrant(audience string) (*auth.AuthorizationGrant, error) {
	data, ok := f.data[audience]
	if !ok {
		return nil, ErrNoAuthenticationData
	}
	return &data.grant, nil
}

func (f *MemoryStore) WhoAmI(audience string) (string, error) {
	data, ok := f.data[audience]
	if !ok {
		return "", ErrNoAuthenticationData
	}
	return data.username, nil
}

func (f *MemoryStore) Logout() error {
	for key := range f.data {
		delete(f.data, key)
	}
	return nil
}

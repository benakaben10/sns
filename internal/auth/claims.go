package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	Email             string            `json:"email,omitempty"`
	PreferredUsername string            `json:"preferred_username,omitempty"`
	Scope             string            `json:"scope,omitempty"`
	RealmAccess       *RealmAccessClaim `json:"realm_access,omitempty"`
	ResourceAccess    map[string]*Roles `json:"resource_access,omitempty"`
}

type RealmAccessClaim struct {
	Roles []string `json:"roles"`
}

type Roles struct {
	Roles []string `json:"roles"`
}

func (c *Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes() {
		if s == scope {
			return true
		}
	}
	return false
}

func (c *Claims) Scopes() []string {
	if c.Scope == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(c.Scope, " ") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (c *Claims) HasRealmRole(role string) bool {
	if c.RealmAccess == nil {
		return false
	}
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (c *Claims) HasResourceRole(resource, role string) bool {
	if c.ResourceAccess == nil {
		return false
	}
	roles, ok := c.ResourceAccess[resource]
	if !ok || roles == nil {
		return false
	}
	for _, r := range roles.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (c *Claims) UserID() string {
	sub, _ := c.GetSubject()
	return sub
}

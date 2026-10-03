package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/auth"
)

// Who may call what is declared once, in the contract: each operation's Security names the scope
// it needs (auth.Needs), and charter's go/auth enforces exactly that (the schemes, the 401s and
// 403s, verifying Access's and an OpenID Connect issuer's JWTs). What is this API's own is the
// scopes, who gets them, and the one rule a scope cannot say (MayPostFor). docs/concepts/auth.md is
// the story.

// The scopes. An operation needs one; a caller is granted some.
const (
	ScopeRead   = "devices:read"   // read every machine
	ScopeWrite  = "devices:write"  // post reports: a machine, for itself
	ScopeForget = "devices:forget" // forget a machine: a person
)

// The two secrets of the bearer tokens, kept for one release while the machines move to Access
// service tokens. Unset, no token matches.
const (
	WriteToken = "WRITE_TOKEN" // posts reports for any machine, reads, forgets
	ReadToken  = "READ_TOKEN"  // reads
)

// Trusted is who may do what:
//   - the read and write tokens (the Worker's secrets), each with its scopes;
//   - Cloudflare Access (ACCESS_TEAM_DOMAIN, ACCESS_AUD): a person who logged in reads and forgets
//     machines; a machine's service token reads, and posts only for the device it was enrolled for
//     (MayPostFor, the machines table);
//   - an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): what each token's scope claim says.
//
// An unset secret or setting trusts no one.
var Trusted = []auth.Trusted{
	auth.Token{Secret: WriteToken, Scopes: []string{ScopeRead, ScopeWrite, ScopeForget}},
	auth.Token{Secret: ReadToken, Scopes: []string{ScopeRead}},
	auth.Access{People: []string{ScopeRead, ScopeForget}, Machines: []string{ScopeRead, ScopeWrite}},
	auth.OIDC{},
}

// MayPostFor is the rule a scope cannot say: a machine's Access service token posts only for the
// device it was enrolled for (the machines table, keyed by the token's Client ID: mise run
// machine:enrol). Any other caller with devices:write posts for any device. A relation check
// (authz-core: may <caller> <relation> <object>?) goes here when there are more such rules.
func (env Env) MayPostFor(ctx context.Context, device string) error {
	caller, ok := auth.CallerOf(ctx)
	if !ok || caller.Machine == "" {
		return nil
	}
	store, err := env.Store()
	if err != nil {
		return err
	}
	enrolled, found, err := store.Machine(ctx, caller.Machine)
	switch {
	case err != nil:
		return err
	case !found:
		return huma.Error403Forbidden(caller.Name() + " was not enrolled for a device: it may not post (mise run machine:enrol)")
	case enrolled != device:
		return huma.Error403Forbidden(caller.Name() + " is this machine's token, for device " + enrolled + ": it may not post for " + device)
	}
	return nil
}

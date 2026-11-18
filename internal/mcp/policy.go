package mcp

import "context"

// ApplyPolicy decides whether apply_change may write to the session environment.
// A nil Protected set means the default, which is prod. An empty non-nil set
// protects nothing.
type ApplyPolicy struct {
	AllowProtectedApply bool
	Protected           map[string]struct{}
}

type applyPolicyKey struct{}

func withApplyPolicy(ctx context.Context, policy ApplyPolicy) context.Context {
	return context.WithValue(ctx, applyPolicyKey{}, policy)
}

func applyPolicyFrom(ctx context.Context) ApplyPolicy {
	if ctx == nil {
		return ApplyPolicy{}
	}
	policy, _ := ctx.Value(applyPolicyKey{}).(ApplyPolicy)
	return policy
}

// IsProtected reports whether name refuses apply_change under this policy.
// An empty name is never protected, so unit tests with no environment keep
// their existing behaviour.
func (p ApplyPolicy) IsProtected(name string) bool {
	if name == "" {
		return false
	}
	set := p.Protected
	if set == nil {
		set = map[string]struct{}{"prod": {}}
	}
	_, ok := set[name]
	return ok
}

package mcp

import "context"

// Actor is the verified caller for one MCP request.
// Roles are informational. SQLForge does not authorize from them.
type Actor struct {
	Subject string
	OrgID   string
	Kind    string
	Roles   []string
	Via     string
}

type actorContextKey struct{}

func contextWithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// ActorFrom returns the actor stored by the MCP server, if any.
func ActorFrom(ctx context.Context) (Actor, bool) {
	if ctx == nil {
		return Actor{}, false
	}
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}

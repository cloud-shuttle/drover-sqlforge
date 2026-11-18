package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlguard"
	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlident"
)

// AuditLog writes one JSON object per line. The line never includes SQL,
// tokens, DSNs, or raw tool arguments.
type AuditLog struct {
	mu sync.Mutex
	w  io.Writer
}

func NewAuditLog(w io.Writer) *AuditLog {
	return &AuditLog{w: w}
}

type auditActor struct {
	Sub  string `json:"sub,omitempty"`
	Org  string `json:"org,omitempty"`
	Kind string `json:"kind,omitempty"`
	Via  string `json:"via,omitempty"`
}

type auditLine struct {
	TS          string     `json:"ts"`
	Event       string     `json:"event"`
	RequestID   string     `json:"request_id,omitempty"`
	Actor       auditActor `json:"actor"`
	Tool        string     `json:"tool"`
	Environment string     `json:"environment"`
	PlanID      string     `json:"plan_id,omitempty"`
	Outcome     string     `json:"outcome"`
	PolicyID    string     `json:"policy_id,omitempty"`
	ErrorClass  string     `json:"error_class,omitempty"`
	DurationMS  int64      `json:"duration_ms"`
	Models      []string   `json:"models,omitempty"`
	AgentJobID  string     `json:"agent_job_id,omitempty"`
}

func (a *AuditLog) WriteLine(line auditLine) {
	if a == nil || a.w == nil {
		return
	}
	raw, err := json.Marshal(line)
	if err != nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.w.Write(append(raw, '\n'))
}

var planIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func safeLog(value string, max int) string {
	if value == "" || len(value) > max || strings.ContainsAny(value, "\r\n\"\\;") {
		return ""
	}
	return value
}

func (s *Server) baseRecord(r *http.Request, actor Actor, tool string) auditLine {
	return auditLine{
		TS:          time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Event:       "sqlforge.mcp.tool_call",
		RequestID:   requestID(r),
		Actor:       auditActor{Sub: safeLog(actor.Subject, 256), Org: safeLog(actor.OrgID, 128), Kind: safeLog(actor.Kind, 32), Via: safeLog(actor.Via, 32)},
		Tool:        tool,
		Environment: s.sessionEnvironment(),
		AgentJobID:  headerJobID(r),
	}
}

func (s *Server) sessionEnvironment() string {
	if s == nil || s.Runtime == nil || s.Runtime.Env == nil {
		return ""
	}
	if sqlident.ValidateEnvName(s.Runtime.Env.Name) != nil {
		return ""
	}
	return s.Runtime.Env.Name
}

func headerJobID(r *http.Request) string {
	if r == nil {
		return ""
	}
	return safeLog(r.Header.Get("x-bf-dim-agent-job-id"), 128)
}

func requestID(r *http.Request) string {
	if r != nil {
		if id := safeLog(r.Header.Get("X-Request-Id"), 128); id != "" {
			return id
		}
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func (s *Server) writeAudit(line auditLine) {
	if s == nil || s.Audit == nil {
		return
	}
	s.Audit.WriteLine(line)
}

func safePlanAndModels(args map[string]any, result any) (string, []string) {
	planID := safePlanID(lookupString(args, "plan_id"))
	models := safeModels(lookupString(args, "model_name"))
	fields, ok := result.(map[string]interface{})
	if !ok {
		return planID, models
	}
	if planID == "" {
		planID = safePlanID(lookupIface(fields, "plan_id"))
	}
	if len(models) == 0 {
		models = safeModels(lookupIface(fields, "model_name"))
	}
	return planID, models
}

func lookupString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return value
}

func lookupIface(fields map[string]interface{}, key string) string {
	value, _ := fields[key].(string)
	return value
}

func safePlanID(id string) string {
	if planIDRe.MatchString(id) {
		return id
	}
	return ""
}

func safeModels(name string) []string {
	if name == "" || sqlident.ValidateIdentifier("model", name) != nil {
		return nil
	}
	return []string{name}
}

func errorClass(err error) string {
	if err == nil {
		return ""
	}
	var guarded *sqlguard.Error
	if errors.As(err, &guarded) {
		return "validation"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "does not match"):
		return "plan_drift"
	case strings.Contains(msg, "protected"):
		return "policy"
	case strings.Contains(msg, "not found"):
		return "not_found"
	default:
		return "error"
	}
}

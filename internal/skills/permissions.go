package skills

import (
	"fmt"
	"strings"
	"time"
)

type PermissionChecker func(permission string) bool

type AuditEvent struct {
	Skill      string
	Tool       string
	Permission string
	Allowed    bool
	Reason     string
	At         time.Time
}

type AuditSink func(event AuditEvent)

func checkPermissions(skillName, toolName string, permissions []string, checker PermissionChecker, audit AuditSink) error {
	perms := normalizePermissions(permissions)
	if len(perms) == 0 {
		return nil
	}
	if checker == nil {
		recordAudit(audit, AuditEvent{
			Skill:   skillName,
			Tool:    toolName,
			Allowed: false,
			Reason:  "permission checker not configured",
		})
		return fmt.Errorf("permission check is not configured")
	}
	for _, perm := range perms {
		allowed := checker(perm)
		recordAudit(audit, AuditEvent{
			Skill:      skillName,
			Tool:       toolName,
			Permission: perm,
			Allowed:    allowed,
		})
		if !allowed {
			return fmt.Errorf("permission denied: %s", perm)
		}
	}
	return nil
}

func normalizePermissions(permissions []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		trimmed := strings.TrimSpace(perm)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func recordAudit(audit AuditSink, event AuditEvent) {
	if audit == nil {
		return
	}
	event.At = time.Now().UTC()
	audit(event)
}

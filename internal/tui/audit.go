package tui

import "github.com/RafaSG13/Pandora/internal/store"

// audit records an action in the audit log (a no-op unless audit_log is on).
// A failure never blocks the action: the first one is kept in auditErr and the
// Update wrapper turns it into a toast once the action's own toast is set.
func (m *model) audit(action, title string) {
	if err := store.RecordAudit(action, title); err != nil && m.auditErr == nil {
		m.auditErr = err
	}
}

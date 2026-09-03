package docsurface

import "context"

// Reset force-restarts the renderer: it tears down the shared browser and all
// tabs so the next Open rebuilds from scratch. The manual escape hatch behind
// the preview_restart tool, for a wedged browser that auto-recovery hasn't yet
// noticed. Safe to call when nothing is open (no-op) or no Chrome exists (the
// next Open returns the usual "renderer unavailable").
func (m *PreviewSessions) Reset(_ context.Context) error {
	m.resetBrowser()
	return nil
}

func (m *PreviewSessions) Close(ctx context.Context, pageID string) error {
	key, err := sessionKey(ctx, pageID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[key]; ok {
		s.tabCancel()
		delete(m.sessions, key)
	}
	return nil
}

func (m *PreviewSessions) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		s.tabCancel()
		delete(m.sessions, k)
	}
	if m.allocCancel != nil {
		m.allocCancel()
		m.allocCancel = nil
		m.allocReady = false
	}
	if m.vendorSrv != nil {
		_ = m.vendorSrv.Close()
		m.vendorSrv = nil
	}
	m.stopReaperOnce.Do(func() { close(m.reaperStop) })
	return nil
}

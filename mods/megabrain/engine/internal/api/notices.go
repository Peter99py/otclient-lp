package api

import (
	"net/http"
	"time"
)

// maxNotices limita quantas notificações recentes são mantidas em memória.
const maxNotices = 50

// Notice é uma notificação recente (em memória; não vai para o banco).
type Notice struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
	At      int64  `json:"at"`
}

// addNotice registra uma notificação recente.
func (s *Server) addNotice(message string) {
	s.noticesMu.Lock()
	defer s.noticesMu.Unlock()
	s.noticeSeq++
	s.notices = append(s.notices, Notice{ID: s.noticeSeq, Message: message, At: time.Now().Unix()})
	if len(s.notices) > maxNotices {
		s.notices = s.notices[len(s.notices)-maxNotices:]
	}
}

// handleNotices devolve as notificações recentes (mais antigas primeiro).
func (s *Server) handleNotices(w http.ResponseWriter, r *http.Request) {
	s.noticesMu.Lock()
	out := make([]Notice, len(s.notices))
	copy(out, s.notices)
	s.noticesMu.Unlock()
	writeJSON(w, out)
}

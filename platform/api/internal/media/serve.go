package media

import "net/http"

// registerDelivery mounts GET/HEAD /media/{id} and /media/{id}/download (WEB-004 part 2).
func (h *Handler) registerDelivery(mux *http.ServeMux) {}

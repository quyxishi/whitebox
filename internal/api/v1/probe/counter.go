package probe

import (
	"io"
	"net/http"
)

type ByteCounter struct {
	io.ReadCloser
	n int64
}

func (h *ByteCounter) Read(p []byte) (int, error) {
	n, err := h.ReadCloser.Read(p)
	h.n += int64(n)
	return n, err
}

type RedirectCounter struct {
	Total int
	Max   int
}

func (h *RedirectCounter) CheckRedirect(req *http.Request, via []*http.Request) error {
	if h.Total = len(via); h.Total > h.Max {
		return http.ErrUseLastResponse
	}
	return nil
}

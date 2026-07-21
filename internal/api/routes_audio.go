package api

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stashapp/stash/pkg/file"
	"github.com/stashapp/stash/pkg/models"
)

type audioRoutes struct {
	routes
	audioFinder models.AudioReader
}

func (rs audioRoutes) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{audioId}/stream", rs.Stream)
	r.Get("/{audioId}/cover", rs.Cover)
	return r
}

func (rs audioRoutes) Stream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "audioId"))
	if err != nil {
		http.Error(w, "invalid audio id", http.StatusBadRequest)
		return
	}
	var a *models.Audio
	err = rs.withReadTxn(r, func(ctx context.Context) error {
		var e error
		a, e = rs.audioFinder.Find(ctx, id)
		if e != nil || a == nil {
			return e
		}
		return a.LoadFiles(ctx, rs.audioFinder)
	})
	if err != nil || a == nil || len(a.Files.List()) == 0 {
		http.NotFound(w, r)
		return
	}
	f := a.Files.List()[0]
	w.Header().Set("Content-Type", "audio/mpeg")
	if err := f.Base().Serve(&file.OsFS{}, w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (rs audioRoutes) Cover(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "audioId"))
	if err != nil {
		http.Error(w, "invalid audio id", http.StatusBadRequest)
		return
	}
	var data []byte
	err = rs.withReadTxn(r, func(ctx context.Context) error { var e error; data, e = rs.audioFinder.GetCover(ctx, id); return e })
	if err != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	http.ServeContent(w, r, "cover", time.Unix(0, 0), bytes.NewReader(data))
}

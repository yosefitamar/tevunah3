package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
)

// Fotos da ocorrência do relatório operacional: a imagem da apreensão, da
// prisão ou do local, que o PDF não traz. O analista anexa na ficha e elas
// vão ao SIPOM em base64, no bloco `fotos` do envio. Mesmas regras de
// formato e tamanho das fotos de dossiê (JPEG ou PNG, até 5 MiB).

type opsPhotoJSON struct {
	ID        string    `json:"id"`
	MIME      string    `json:"mime"`
	Size      int       `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func toOpsPhotosJSON(photos []opsreport.Photo) []opsPhotoJSON {
	out := make([]opsPhotoJSON, 0, len(photos))
	for _, p := range photos {
		out = append(out, opsPhotoJSON{ID: p.ID, MIME: p.MIME, Size: p.Size, CreatedAt: p.CreatedAt})
	}
	return out
}

func (a *app) auditOpsPhoto(r *http.Request, action, occID string, before, after map[string]any) {
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       action,
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &occID,
		Before:       before,
		After:        after,
	})
}

// POST /api/ops-occurrences/{id}/photos
//
// Multipart, campo "photo". Devolve a lista de fotos da ocorrência já com a
// nova.
func (a *app) handleOpsPhotoUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	r.Body = http.MaxBytesReader(w, r.Body, photoMaxBytes)
	if err := r.ParseMultipartForm(photoMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 5 MiB")
		return
	}
	file, header, err := r.FormFile(photoFieldName)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo de foto ausente (campo 'photo')")
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	mime := http.DetectContentType(head)
	ext := extForMime(mime)
	if ext == "" {
		httpx.Error(w, http.StatusBadRequest,
			"formato não suportado — envie JPEG ou PNG (recebido: "+header.Header.Get("Content-Type")+")")
		return
	}

	dir := photoDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("mkdir photo_dir %s: %v", dir, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao preparar storage")
		return
	}
	// Prefixo "ops-" para namespacear dentro do PHOTO_DIR.
	filename := "ops-" + uuid.NewString() + ext
	dst := filepath.Join(dir, filename)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		log.Printf("abrir destino foto da ocorrência: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}
	size, err := out.Write(head)
	if err == nil {
		var rest int64
		rest, err = io.Copy(out, file)
		size += int(rest)
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("gravar foto da ocorrência: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}

	p, err := a.opsReports.AddPhoto(r.Context(), id, filename, mime, size, me.ID)
	if err != nil {
		_ = os.Remove(dst)
		switch {
		case errors.Is(err, opsreport.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		case errors.Is(err, opsreport.ErrPhotoLimit):
			httpx.Error(w, http.StatusConflict,
				fmt.Sprintf("a ocorrência já tem %d fotos — remova uma para anexar outra", opsreport.MaxPhotos))
		default:
			log.Printf("opsreport add photo: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao registrar foto")
		}
		return
	}
	a.auditOpsPhoto(r, "opsreport.photo.add", id, nil,
		map[string]any{"photo_id": p.ID, "photo_path": p.Path, "mime": p.MIME, "size": p.Size})

	photos, err := a.opsReports.ListPhotos(r.Context(), id)
	if err != nil {
		log.Printf("opsreport list photos: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar as fotos")
		return
	}
	httpx.Created(w, map[string]any{"photos": toOpsPhotosJSON(photos)})
}

// GET /api/ops-occurrences/{id}/photos/{pid}
func (a *app) handleOpsPhotoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	p, err := a.opsReports.FindPhoto(r.Context(), r.PathValue("id"), r.PathValue("pid"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "foto não encontrada")
		return
	}
	path := filepath.Join(photoDir(), p.Path)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "foto não encontrada")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// DELETE /api/ops-occurrences/{id}/photos/{pid}
func (a *app) handleOpsPhotoDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	p, err := a.opsReports.DeletePhoto(r.Context(), id, r.PathValue("pid"), me.ID)
	if err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.NoContent(w)
			return
		}
		log.Printf("opsreport delete photo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}
	_ = os.Remove(filepath.Join(photoDir(), p.Path))
	a.auditOpsPhoto(r, "opsreport.photo.delete", id,
		map[string]any{"photo_id": p.ID, "photo_path": p.Path, "mime": p.MIME, "size": p.Size}, nil)
	httpx.NoContent(w)
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// ─────────────────────────── POST /api/entities/{id}/photos ───────────────
//
// Anexa uma foto adicional (galeria) à entidade. Multipart: campo "photo"
// (binário) + opcionais "caption" (text), "ord" (int). Mesmas regras de
// formato/tamanho da foto primária.

func (a *app) handleEntityGalleryUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	current, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("entities find pre-gallery: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Classification > me.ClearanceLevel || current.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

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

	caption := strings.TrimSpace(r.FormValue("caption"))
	if len(caption) > 500 {
		httpx.Error(w, http.StatusBadRequest, "legenda excede 500 caracteres")
		return
	}
	ord := 0
	if v := strings.TrimSpace(r.FormValue("ord")); v != "" {
		// Parse silencioso: campo é opcional. Se inválido, mantém 0.
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			ord = n
		}
	}

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

	// Galeria usa filename próprio "<photoUUID>.<ext>" — não colide com a foto
	// primária ("<entityUUID>.<ext>"), e dois uploads consecutivos não se
	// sobrescrevem como acontece com a primária.
	photoUUID := uuid.NewString()
	filename := photoUUID + ext
	dir := photoDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("mkdir photo_dir %s: %v", dir, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao preparar storage")
		return
	}
	dst := filepath.Join(dir, filename)

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		log.Printf("abrir destino galeria: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}
	defer out.Close()

	if _, err := out.Write(head); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}

	g, err := a.entities.AddGalleryPhoto(r.Context(), entities.NewGalleryPhoto{
		EntityID:  id,
		PhotoPath: filename,
		Caption:   caption,
		MIME:      mime,
	}, ord, me.ID)
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("add gallery photo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar foto")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := current.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.gallery.add",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After: map[string]any{
			"photo_id":   g.ID,
			"photo_path": g.PhotoPath,
			"caption":    g.Caption,
			"mime":       g.MIME,
			"ord":        g.Ord,
		},
	})

	httpx.Created(w, map[string]any{"photo": toPublicPhoto(g)})
}

// ─────────────────────────── GET /api/entities/{id}/photos/{pid} ──────────
//
// Serve o binário de uma foto da galeria. Permissão entity.read + clearance.

func (a *app) handleEntityGalleryGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	id := r.PathValue("id")
	pid := r.PathValue("pid")
	if id == "" || pid == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	e, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if e.Classification > me.ClearanceLevel || e.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	g, err := a.entities.FindGalleryPhoto(r.Context(), id, pid)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "foto não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}

	path := filepath.Join(photoDir(), g.PhotoPath)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "foto não encontrada")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// ─────────────────────────── PATCH /api/entities/{id}/photos/{pid} ────────
//
// Atualiza apenas metadados (caption, ord). O binário em si não é alterável
// — re-upload (DELETE + POST) se quiser trocar o arquivo.

type galleryPatchReq struct {
	Caption *string `json:"caption"`
	Ord     *int    `json:"ord"`
}

func (a *app) handleEntityGalleryPatch(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	pid := r.PathValue("pid")
	if id == "" || pid == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	current, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Classification > me.ClearanceLevel || current.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	var body galleryPatchReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if body.Caption == nil {
		httpx.Error(w, http.StatusBadRequest, "campo 'caption' obrigatório")
		return
	}
	caption := strings.TrimSpace(*body.Caption)
	if len(caption) > 500 {
		httpx.Error(w, http.StatusBadRequest, "legenda excede 500 caracteres")
		return
	}

	g, err := a.entities.UpdateGalleryCaption(r.Context(), id, pid, caption, body.Ord, me.ID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "foto não encontrada")
			return
		}
		log.Printf("update gallery caption: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar foto")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := current.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.gallery.update",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After: map[string]any{
			"photo_id": g.ID,
			"caption":  g.Caption,
			"ord":      g.Ord,
		},
	})

	httpx.OK(w, map[string]any{"photo": toPublicPhoto(g)})
}

// ─────────────────────────── DELETE /api/entities/{id}/photos/{pid} ───────
//
// Soft-delete + remoção física do arquivo. O registro persiste para
// auditoria mas o blob sai do disco.

func (a *app) handleEntityGalleryDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	pid := r.PathValue("pid")
	if id == "" || pid == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	current, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Classification > me.ClearanceLevel || current.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	oldPath, err := a.entities.SoftDeleteGalleryPhoto(r.Context(), id, pid, me.ID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.NoContent(w)
			return
		}
		log.Printf("delete gallery photo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}
	if oldPath != "" {
		_ = os.Remove(filepath.Join(photoDir(), oldPath))
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := current.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.gallery.delete",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		Before: map[string]any{
			"photo_id":   pid,
			"photo_path": oldPath,
		},
	})
	httpx.NoContent(w)
}

func toPublicPhoto(g *entities.GalleryPhoto) publicPhoto {
	return publicPhoto{
		ID:        g.ID,
		Caption:   g.Caption,
		MIME:      g.MIME,
		Ord:       g.Ord,
		CreatedAt: g.CreatedAt,
		CreatedBy: g.CreatedBy,
		UpdatedAt: g.UpdatedAt,
		UpdatedBy: g.UpdatedBy,
	}
}

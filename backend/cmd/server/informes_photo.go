package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/informes"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// ─────────────────────────── POST /api/informes/{id}/photo ─────────────────

func (a *app) handleInformePhotoUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	cur, err := a.informes.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	if !canManageInforme(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode anexar foto")
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
	// Prefixo "informe-" pra namespacear dentro do PHOTO_DIR.
	filename := "informe-" + id + ext
	dst := filepath.Join(dir, filename)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir destino foto informe: %v", err)
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

	oldPath, err := a.informes.SetPhotoPath(r.Context(), id, filename, me.ID)
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("set informe photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar foto")
		return
	}
	if oldPath != "" && oldPath != filename {
		_ = os.Remove(filepath.Join(dir, oldPath))
	}

	a.auditInforme(r, "informe.photo.set", id, cur.RequiredClearance, nil,
		map[string]any{"photo_path": filename, "mime": mime})

	inf, err := a.informes.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar informe")
		return
	}
	httpx.OK(w, map[string]any{"informe": toPublicInforme(inf)})
}

// ─────────────────────────── GET /api/informes/{id}/photo ──────────────────

func (a *app) handleInformePhotoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	ok, err := a.informes.CanAccess(r.Context(), id, me.ID, me.ClearanceLevel, hasRole(me.Roles, "administrador"))
	if err != nil || !ok {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	inf, err := a.informes.FindByID(r.Context(), id)
	if err != nil || inf.PhotoPath == nil || *inf.PhotoPath == "" {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}
	path := filepath.Join(photoDir(), *inf.PhotoPath)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}
	a.auditInforme(r, "informe.photo.view", id, inf.RequiredClearance, nil, nil)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// ─────────────────────────── DELETE /api/informes/{id}/photo ───────────────

func (a *app) handleInformePhotoDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	cur, err := a.informes.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	if !canManageInforme(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode remover a foto")
		return
	}
	if cur.PhotoPath == nil || *cur.PhotoPath == "" {
		httpx.NoContent(w)
		return
	}
	oldPath, err := a.informes.SetPhotoPath(r.Context(), id, "", me.ID)
	if err != nil {
		if errors.Is(err, informes.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "informe não encontrado")
			return
		}
		log.Printf("clear informe photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}
	if oldPath != "" {
		_ = os.Remove(filepath.Join(photoDir(), oldPath))
	}
	a.auditInforme(r, "informe.photo.delete", id, cur.RequiredClearance,
		map[string]any{"photo_path": oldPath}, nil)
	httpx.NoContent(w)
}

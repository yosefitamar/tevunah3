package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/belia/tevunah/backend/internal/consultas"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// Print do pedido: a captura de tela da mensagem, do e-mail ou do ofício em
// que o solicitante pediu a consulta. Opcional, um por registro.

// ─────────────────────────── POST /api/consultas/{id}/print ────────────────

func (a *app) handleConsultaPrintUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	cur := a.findVisibleConsulta(w, r)
	if cur == nil {
		return
	}
	if !canManageConsulta(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode anexar o print")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, photoMaxBytes)
	if err := r.ParseMultipartForm(photoMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 5 MiB")
		return
	}
	file, header, err := r.FormFile(photoFieldName)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo do print ausente (campo 'photo')")
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
	// Prefixo "consulta-" pra namespacear dentro do PHOTO_DIR.
	filename := "consulta-" + cur.ID + ext
	dst := filepath.Join(dir, filename)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir destino print consulta: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar o print")
		return
	}
	defer out.Close()
	if _, err := out.Write(head); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar o print")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar o print")
		return
	}

	oldPath, err := a.consultas.SetPrintPath(r.Context(), cur.ID, filename, me.ID)
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("set consulta print_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar o print")
		return
	}
	if oldPath != "" && oldPath != filename {
		_ = os.Remove(filepath.Join(dir, oldPath))
	}

	var before map[string]any
	if oldPath != "" {
		before = map[string]any{"print_path": oldPath}
	}
	a.auditConsulta(r, "consulta.print.set", cur.ID, before,
		map[string]any{"print_path": filename, "mime": mime})

	c, err := a.consultas.FindByID(r.Context(), cur.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar o registro")
		return
	}
	httpx.OK(w, map[string]any{"consulta": a.toPublicConsulta(c)})
}

// ─────────────────────────── GET /api/consultas/{id}/print ─────────────────

func (a *app) handleConsultaPrintGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.read") {
		return
	}
	c := a.findVisibleConsulta(w, r)
	if c == nil {
		return
	}
	if c.PrintPath == nil {
		httpx.Error(w, http.StatusNotFound, "sem print registrado")
		return
	}
	path := filepath.Join(photoDir(), *c.PrintPath)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "sem print registrado")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// ─────────────────────────── DELETE /api/consultas/{id}/print ──────────────

func (a *app) handleConsultaPrintDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	cur := a.findVisibleConsulta(w, r)
	if cur == nil {
		return
	}
	if !canManageConsulta(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode remover o print")
		return
	}
	if cur.PrintPath == nil {
		httpx.NoContent(w)
		return
	}
	oldPath, err := a.consultas.SetPrintPath(r.Context(), cur.ID, "", me.ID)
	if err != nil {
		if errors.Is(err, consultas.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "registro de consulta não encontrado")
			return
		}
		log.Printf("clear consulta print_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover o print")
		return
	}
	if oldPath != "" {
		_ = os.Remove(filepath.Join(photoDir(), oldPath))
	}
	a.auditConsulta(r, "consulta.print.delete", cur.ID,
		map[string]any{"print_path": oldPath}, nil)
	httpx.NoContent(w)
}

package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// Limites e formatos aceitos para upload de foto. O recorte 3:4 fica a cargo
// do front (cliente envia o JPEG/PNG já cortado). O backend só valida tipo
// pelo conteúdo (magic bytes) e tamanho.
const (
	photoMaxBytes  = 5 << 20 // 5 MiB
	photoFieldName = "photo"
)

// photoDir devolve o diretório onde os arquivos de foto são gravados, lendo
// PHOTO_DIR (default: /var/lib/tevunah/photos).
func photoDir() string {
	if v := strings.TrimSpace(os.Getenv("PHOTO_DIR")); v != "" {
		return v
	}
	return "/var/lib/tevunah/photos"
}

// ─────────────────────────── POST /api/entities/{id}/photo ───────────────

func (a *app) handleEntityPhotoUpload(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("entities find pre-photo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Kind != entities.KindPerson {
		httpx.Error(w, http.StatusBadRequest, "fotos só se aplicam a entidades do tipo pessoa")
		return
	}
	if current.Classification > me.ClearanceLevel || current.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	// Limita o tamanho do body antes de processar.
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

	// Lê o cabeçalho do arquivo (512 bytes) para detectar o tipo real.
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

	// Garante o diretório de fotos e abre o arquivo destino.
	dir := photoDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("mkdir photo_dir %s: %v", dir, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao preparar storage")
		return
	}
	filename := id + ext
	dst := filepath.Join(dir, filename)

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir arquivo destino: %v", err)
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

	// Atualiza o registro. Se havia foto anterior com outra extensão, removemos.
	oldPath, err := a.entities.SetPhotoPath(r.Context(), id, filename, me.ID)
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("set photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar foto")
		return
	}
	if oldPath != "" && oldPath != filename {
		_ = os.Remove(filepath.Join(dir, oldPath))
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := current.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.photo.set",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After:                  map[string]any{"photo_path": filename, "mime": mime},
	})

	updated, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar entidade")
		return
	}
	httpx.OK(w, map[string]any{"entity": toPublicEntity(updated)})
}

// ─────────────────────────── GET /api/entities/{id}/photo ────────────────

func (a *app) handleEntityPhotoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
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
	if e.Kind != entities.KindPerson || e.Person == nil ||
		e.Person.PhotoPath == nil || *e.Person.PhotoPath == "" {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}
	path := filepath.Join(photoDir(), *e.Person.PhotoPath)
	// Defesa contra path traversal: o filename armazenado deve ser exatamente
	// o que confiamos. Verifica que o caminho resolvido está sob photoDir().
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// ─────────────────────────── DELETE /api/entities/{id}/photo ─────────────

func (a *app) handleEntityPhotoDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
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
	if e.Kind != entities.KindPerson || e.Person == nil ||
		e.Person.PhotoPath == nil || *e.Person.PhotoPath == "" {
		httpx.NoContent(w)
		return
	}

	oldPath, err := a.entities.SetPhotoPath(r.Context(), id, "", me.ID)
	if err != nil {
		log.Printf("clear photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}
	if oldPath != "" {
		_ = os.Remove(filepath.Join(photoDir(), oldPath))
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := e.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.photo.delete",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		Before:                 map[string]any{"photo_path": oldPath},
	})
	httpx.NoContent(w)
}

// extForMime devolve a extensão padrão para o MIME detectado, ou "" se não
// for um formato aceito (JPEG/PNG).
func extForMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	}
	return ""
}

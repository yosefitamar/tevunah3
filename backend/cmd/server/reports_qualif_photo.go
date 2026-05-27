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
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/reports"
)

// Fotos de qualificação seguem o mesmo storage das fotos de entidade
// (photoDir() compartilhado), com prefixo "qual_" para distinguir.
// Só faz sentido em qualificações 'militar' — civis usam a foto da entidade.

// POST /api/reports/{id}/qualifications/{qid}/photo
func (a *app) handleQualificationPhotoUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.update") {
		return
	}
	reportID := r.PathValue("id")
	qid := r.PathValue("qid")

	q, err := a.reports.FindQualification(r.Context(), qid)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "qualificação não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar qualificação")
		return
	}
	if q.ReportID != reportID {
		httpx.Error(w, http.StatusNotFound, "qualificação não encontrada")
		return
	}
	rep, err := a.reports.FindByID(r.Context(), reportID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar relatório")
		return
	}
	if rep.Status != reports.StatusCriado {
		httpx.Error(w, http.StatusConflict, "relatório não está em status 'criado'")
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
	filename := "qual_" + qid + ext
	dst := filepath.Join(dir, filename)

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir arquivo destino qualif: %v", err)
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

	oldPath, err := a.reports.SetQualificationPhotoPath(r.Context(), qid, filename)
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("set qualif photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar foto")
		return
	}
	if oldPath != "" && oldPath != filename {
		_ = os.Remove(filepath.Join(dir, oldPath))
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.qualification.photo.set",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(reportID),
		After:        map[string]any{"qualification_id": qid, "photo_path": filename, "mime": mime},
	})

	updated, err := a.reports.FindQualification(r.Context(), qid)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar qualificação")
		return
	}
	httpx.OK(w, map[string]any{"qualification": toPublicQualification(updated)})
}

// GET /api/reports/{id}/qualifications/{qid}/photo
func (a *app) handleQualificationPhotoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.read") {
		return
	}
	reportID := r.PathValue("id")
	qid := r.PathValue("qid")

	q, err := a.reports.FindQualification(r.Context(), qid)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "qualificação não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if q.ReportID != reportID {
		httpx.Error(w, http.StatusNotFound, "qualificação não encontrada")
		return
	}
	pp, _ := q.Data["photo_path"].(string)
	if pp == "" {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}
	path := filepath.Join(photoDir(), pp)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.qualification.photo.view",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(reportID),
	})

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

// DELETE /api/reports/{id}/qualifications/{qid}/photo
func (a *app) handleQualificationPhotoDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.update") {
		return
	}
	reportID := r.PathValue("id")
	qid := r.PathValue("qid")

	q, err := a.reports.FindQualification(r.Context(), qid)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "qualificação não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if q.ReportID != reportID {
		httpx.Error(w, http.StatusNotFound, "qualificação não encontrada")
		return
	}
	rep, err := a.reports.FindByID(r.Context(), reportID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar relatório")
		return
	}
	if rep.Status != reports.StatusCriado {
		httpx.Error(w, http.StatusConflict, "relatório não está em status 'criado'")
		return
	}

	oldPath, err := a.reports.SetQualificationPhotoPath(r.Context(), qid, "")
	if err != nil {
		log.Printf("clear qualif photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}
	if oldPath != "" {
		_ = os.Remove(filepath.Join(photoDir(), oldPath))
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.qualification.photo.delete",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(reportID),
		Before:       map[string]any{"qualification_id": qid, "photo_path": oldPath},
	})
	httpx.NoContent(w)
}

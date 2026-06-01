package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// Filenames canônicos dos assets institucionais dentro de PHOTO_DIR. Casam com
// os nomes que o pdf.go carrega via loadAsset, então uploadar pela UI substitui
// o arquivo já consumido pelo gerador de PDF.
const (
	brasaoFilenameBase    = "logo-sai"           // brasão da agência (SAI)
	institutionalLogoBase = "logo-instituicao2"  // logos institucionais (PMCE + CEARÁ)
)

const brasaoMaxBytes = 2 << 20 // 2 MiB

// writeUploadedImage lê o arquivo multipart do campo `field`, valida que é
// PNG/JPEG e grava em PHOTO_DIR/<base>.<ext>, limpando as outras extensões pra
// não deixar resíduo (o loadAsset tenta .png/.jpg/.jpeg na ordem). Em erro,
// escreve a resposta HTTP e devolve ok=false; em sucesso devolve (filename, mime).
func (a *app) writeUploadedImage(w http.ResponseWriter, r *http.Request, field, base string) (string, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, brasaoMaxBytes)
	if err := r.ParseMultipartForm(brasaoMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 2 MiB")
		return "", "", false
	}
	file, header, err := r.FormFile(field)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo ausente (campo '"+field+"')")
		return "", "", false
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
		return "", "", false
	}

	dir := photoDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("mkdir photo_dir %s: %v", dir, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao preparar storage")
		return "", "", false
	}
	for _, oldExt := range []string{".png", ".jpg", ".jpeg"} {
		if oldExt == ext {
			continue
		}
		_ = os.Remove(filepath.Join(dir, base+oldExt))
	}

	filename := base + ext
	dst := filepath.Join(dir, filename)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir destino %s: %v", base, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar imagem")
		return "", "", false
	}
	defer out.Close()
	if _, err := out.Write(head); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar imagem")
		return "", "", false
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar imagem")
		return "", "", false
	}
	return filename, mime, true
}

// serveAsset serve PHOTO_DIR/<base>.{png,jpg,jpeg} (preview no admin). 404 se
// nenhuma extensão existir.
func (a *app) serveAsset(w http.ResponseWriter, r *http.Request, base string) {
	dir := photoDir()
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		p := filepath.Join(dir, base+ext)
		if _, err := os.Stat(p); err == nil {
			http.ServeFile(w, r, p)
			return
		}
	}
	httpx.Error(w, http.StatusNotFound, "imagem não configurada")
}

type publicSettings struct {
	AgencyName    string `json:"agency_name"`
	DocumentTitle string `json:"document_title"`
	BrasaoPath    string `json:"brasao_path,omitempty"`
}

// GET /api/system-settings — endpoint público (não exige autenticação) pra
// que o LoginScreen consiga exibir o nome da agência ANTES do login. Os
// dados expostos (agency_name, document_title, brasao_path) são cosméticos —
// o conteúdo binário do brasão segue protegido em outro endpoint.
func (a *app) handleSystemSettingsGet(w http.ResponseWriter, r *http.Request) {
	s, err := a.settings.Get(r.Context())
	if err != nil {
		log.Printf("settings get: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler configurações")
		return
	}
	httpx.OK(w, map[string]any{"settings": publicSettings{
		AgencyName:    s.AgencyName,
		DocumentTitle: s.DocumentTitle,
		BrasaoPath:    s.BrasaoPath,
	}})
}

type updateSettingsRequest struct {
	AgencyName    string `json:"agency_name"`
	DocumentTitle string `json:"document_title"`
}

// PUT /api/admin/system-settings — atualiza texto (agency + título). Brasão
// vai por outro endpoint (multipart).
func (a *app) handleSystemSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "system.settings.update") {
		return
	}
	var req updateSettingsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	agency := strings.TrimSpace(req.AgencyName)
	title := strings.TrimSpace(req.DocumentTitle)
	if agency == "" {
		httpx.Error(w, http.StatusBadRequest, "agency_name não pode ser vazio")
		return
	}
	me := middleware.UserFrom(r.Context())
	before, err := a.settings.Get(r.Context())
	if err != nil {
		log.Printf("settings get pre-update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler configurações")
		return
	}
	if err := a.settings.UpdateText(r.Context(), agency, title, me.ID); err != nil {
		log.Printf("settings update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: "system.settings.update",
		Before: map[string]any{"agency_name": before.AgencyName, "document_title": before.DocumentTitle},
		After:  map[string]any{"agency_name": agency, "document_title": title},
	})
	httpx.OK(w, map[string]any{"settings": publicSettings{
		AgencyName:    agency,
		DocumentTitle: title,
		BrasaoPath:    before.BrasaoPath,
	}})
}

// PUT /api/admin/system-settings/brasao — upload do brasão (multipart).
// Grava em PHOTO_DIR/logo-sai.<ext> pra o gerador de PDF (pdf.go:loadAsset)
// reutilizar sem qualquer outra mudança.
func (a *app) handleSystemSettingsBrasaoUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "system.settings.update") {
		return
	}
	filename, mime, ok := a.writeUploadedImage(w, r, "brasao", brasaoFilenameBase)
	if !ok {
		return
	}

	me := middleware.UserFrom(r.Context())
	if err := a.settings.SetBrasaoPath(r.Context(), filename, me.ID); err != nil {
		_ = os.Remove(filepath.Join(photoDir(), filename))
		log.Printf("settings set brasao path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar brasão")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: "system.settings.brasao.set",
		After:  map[string]any{"brasao_path": filename, "mime": mime},
	})

	httpx.OK(w, map[string]any{"brasao_path": filename, "mime": mime})
}

// PUT /api/admin/system-settings/logo — upload do logo institucional
// (PMCE + CEARÁ). Grava em PHOTO_DIR/logo-instituicao2.<ext>; o gerador de PDF
// consome via loadAsset("logo-instituicao2"). Não usa coluna em settings —
// a presença do arquivo no PHOTO_DIR é a fonte de verdade.
func (a *app) handleSystemSettingsLogoUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "system.settings.update") {
		return
	}
	filename, mime, ok := a.writeUploadedImage(w, r, "logo", institutionalLogoBase)
	if !ok {
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: "system.settings.logo.set",
		After:  map[string]any{"logo_path": filename, "mime": mime},
	})

	httpx.OK(w, map[string]any{"logo_path": filename, "mime": mime})
}

// GET /api/admin/system-settings/logo — serve o logo institucional p/ preview.
func (a *app) handleSystemSettingsLogoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "system.settings.update") {
		return
	}
	a.serveAsset(w, r, institutionalLogoBase)
}

// GET /api/admin/system-settings/brasao — serve o arquivo do brasão atual
// para preview no painel de admin. Não exposto a outros papéis (a UI geral
// não exibe o brasão; ele só vai pro PDF).
func (a *app) handleSystemSettingsBrasaoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "system.settings.update") {
		return
	}
	s, err := a.settings.Get(r.Context())
	if err != nil || s.BrasaoPath == "" {
		httpx.Error(w, http.StatusNotFound, "brasão não configurado")
		return
	}
	if strings.ContainsAny(s.BrasaoPath, "/\\") {
		httpx.Error(w, http.StatusNotFound, "brasão não configurado")
		return
	}
	path := filepath.Join(photoDir(), s.BrasaoPath)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "brasão não configurado")
		return
	}
	http.ServeFile(w, r, abs)
}

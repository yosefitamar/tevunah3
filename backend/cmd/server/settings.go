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

// brasaoFilenameBase é o filename canônico do brasão dentro de PHOTO_DIR.
// Mantém compatibilidade com pdf.go (que carrega "logo-sai" via loadAsset),
// então uploadar pela UI substitui o arquivo já consumido pelo gerador de PDF.
const brasaoFilenameBase = "logo-sai"

const brasaoMaxBytes = 2 << 20 // 2 MiB

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
	r.Body = http.MaxBytesReader(w, r.Body, brasaoMaxBytes)
	if err := r.ParseMultipartForm(brasaoMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 2 MiB")
		return
	}
	file, header, err := r.FormFile("brasao")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo ausente (campo 'brasao')")
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

	// Apaga eventuais brasões antigos em outras extensões pra evitar resíduo.
	// O gerador de PDF tenta .png, .jpg, .jpeg na ordem — sem cleanup poderíamos
	// servir o brasão anterior caso o novo seja .png mas o antigo .jpg ainda exista.
	for _, oldExt := range []string{".png", ".jpg", ".jpeg"} {
		if oldExt == ext {
			continue
		}
		_ = os.Remove(filepath.Join(dir, brasaoFilenameBase+oldExt))
	}

	filename := brasaoFilenameBase + ext
	dst := filepath.Join(dir, filename)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir destino brasão: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar brasão")
		return
	}
	defer out.Close()
	if _, err := out.Write(head); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar brasão")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar brasão")
		return
	}

	me := middleware.UserFrom(r.Context())
	if err := a.settings.SetBrasaoPath(r.Context(), filename, me.ID); err != nil {
		_ = os.Remove(dst)
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

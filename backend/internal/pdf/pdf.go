// Package pdf gera PDFs do RELINT via wkhtmltopdf (binário standalone que
// usa Qt WebKit). Escolhemos wkhtmltopdf em vez de Chromium/Gotenberg porque
// o template Laravel original do RELINT foi escrito pra ele (truque do
// `margin-top: -45mm` no header é assinatura do wkhtmltopdf), e o Chromium
// tem comportamento imprevisível com position:fixed em PDF print mode.
package pdf

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// Client é o cliente do wkhtmltopdf. Não tem mais URL/HTTP porque é binário
// local; mantém o photoDir pra inlining de imagens das qualificações.
type Client struct {
	binary   string
	photoDir string
}

// New cria um cliente. baseURL é ignorado (assinatura preservada por
// compatibilidade — Gotenberg URL não importa mais). photoDir é onde ficam
// as fotos das qualificações e os assets institucionais.
func New(_ string, photoDir string) *Client {
	return &Client{
		binary:   "wkhtmltopdf",
		photoDir: photoDir,
	}
}

// ReportData é tudo que o template precisa pra renderizar.
type ReportData struct {
	Number          string
	Status          string
	IsDraft         bool
	DocDateBR       string // dd/mm/aaaa
	DocDateShort    string // DDMmmAA
	Subject         string
	Origin          string
	Diffusion       string
	PriorDiffusion  string
	Reference       string
	Attachments     string
	BodyHTML        template.HTML
	Qualifications  []QualificationData
	DownloadID      string
	GeneratedAtBR   string
	GeneratedByName string
	AgentCode       string // código do agente que baixou (ex.: "ANL-0042")

	// Confidentiality em formato display ("S I G I L O S O", "S E C R E T O",
	// "U L T R A S S E C R E T O") — string já formatada com espaços largos
	// pra encaixar no visual atual do RELINT.
	Classification string

	TitleLine           string
	QRCodeData          template.URL
	BrasaoSAIData       template.URL
	BrasaoInstitucional template.URL
	FooterRibbonData    template.URL
}

// QualificationData é uma qualificação pronta pra render.
type QualificationData struct {
	Kind      string
	Name      string
	Source    string
	PhotoData template.URL
	Fields    []KV
}

// KV é um par chave→valor de campo de qualificação.
type KV struct{ K, V string }

// Render produz o PDF do relatório.
func (c *Client) Render(ctx context.Context, d ReportData) ([]byte, error) {
	d.TitleLine = titleLine(d)
	// QR codifica a linha de título (mesmo conteúdo da barra) + o código do
	// agente que baixou — vínculo forense que sobrevive a foto, print e scan.
	// ECC High pra resiliência em re-prints/desgaste.
	qrPayload := d.TitleLine
	if d.AgentCode != "" {
		qrPayload += " - " + d.AgentCode
	}
	if png, err := qrcode.Encode(qrPayload, qrcode.High, 256); err == nil {
		d.QRCodeData = template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
	}
	d.BrasaoSAIData = template.URL(c.loadAsset("logo-sai"))
	d.BrasaoInstitucional = template.URL(c.loadAsset("logo-instituicao2"))
	d.FooterRibbonData = template.URL(c.loadAsset("footer"))

	// Renderiza os três HTMLs: corpo + header + footer.
	var bodyBuf, headerBuf, footerBuf bytes.Buffer
	if err := indexTmpl.Execute(&bodyBuf, d); err != nil {
		return nil, fmt.Errorf("render body: %w", err)
	}
	if err := headerTmpl.Execute(&headerBuf, d); err != nil {
		return nil, fmt.Errorf("render header: %w", err)
	}
	if err := footerTmpl.Execute(&footerBuf, d); err != nil {
		return nil, fmt.Errorf("render footer: %w", err)
	}

	// wkhtmltopdf precisa de arquivos para --header-html / --footer-html.
	// Cria um diretório temporário e grava os três HTMLs.
	tmpDir, err := os.MkdirTemp("", "relint-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	headerPath := filepath.Join(tmpDir, "header.html")
	footerPath := filepath.Join(tmpDir, "footer.html")
	if err := os.WriteFile(headerPath, headerBuf.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("write header.html: %w", err)
	}
	if err := os.WriteFile(footerPath, footerBuf.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("write footer.html: %w", err)
	}

	// Flags do wkhtmltopdf:
	//   --margin-top/bottom: espaço reservado pra header/footer em cada pg
	//   --header-html / --footer-html: arquivos repetidos em toda página
	//   --header-spacing / --footer-spacing: distância entre conteúdo e bordas
	// marginLeft/marginRight=0 dá aos canvas de --header-html e --footer-html
	// largura física total da A4 (210mm), necessário pro ribbon ser full
	// bleed. O recuo lateral de 20mm do conteúdo é aplicado via CSS dentro
	// de cada HTML (padding/margin em wrappers `safe`).
	args := []string{
		"--page-size", "A4",
		"--margin-top", "50mm",
		// 40mm cabia o footer mas deixava a última linha do corpo colada na
		// borda da caixa do aviso (lei 12.527). 50mm reserva mais espaço; a
		// caixa do aviso é empurrada pra baixo dentro do footer via
		// margin-top em .safe (template), gerando ~10mm de respiro entre o
		// fim do corpo e o topo da caixa.
		"--margin-bottom", "50mm",
		"--margin-left", "0",
		"--margin-right", "0",
		"--header-html", headerPath,
		"--header-spacing", "0",
		"--footer-html", footerPath,
		"--footer-spacing", "0",
		"--encoding", "utf-8",
		"--print-media-type",
		"--disable-smart-shrinking",
		"--enable-local-file-access",
		// JS habilitado + delay pra o script onload do header.html rodar
		// (detecta page=1 pela URL e aplica borda só na primeira página).
		"--enable-javascript",
		"--javascript-delay", "200",
		"--no-stop-slow-scripts",
		"--quiet",
		"-", // stdin: corpo
		"-", // stdout: pdf
	}
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Stdin = &bodyBuf
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("wkhtmltopdf: %s", msg)
	}
	return out.Bytes(), nil
}

// titleLine compõe a linha da caixa de título.
func titleLine(d ReportData) string {
	num := d.Number
	if num == "" {
		num = "RASCUNHO"
	}
	origin := strings.TrimSpace(d.Origin)
	if origin == "" {
		origin = "—"
	}
	return fmt.Sprintf("RELATÓRIO INTERNO Nº %s – %s – %s", num, origin, d.DocDateBR)
}

// loadAsset carrega PHOTO_DIR/<base>.{png,jpg,jpeg} como data URI base64.
func (c *Client) loadAsset(base string) string {
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		raw, err := os.ReadFile(filepath.Join(c.photoDir, base+ext))
		if err != nil {
			continue
		}
		mime := http.DetectContentType(raw)
		if mime != "image/png" && mime != "image/jpeg" {
			continue
		}
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
	}
	return ""
}

// InlinePhoto carrega uma foto específica do photoDir como data URI.
func (c *Client) InlinePhoto(filename string) string {
	if filename == "" || c.photoDir == "" {
		return ""
	}
	if strings.ContainsAny(filename, "/\\") {
		return ""
	}
	path := filepath.Join(c.photoDir, filename)
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(abs, c.photoDir) {
		return ""
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	mime := http.DetectContentType(raw)
	if mime != "image/jpeg" && mime != "image/png" {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}

package pdf

import "html/template"

// Três templates separados para wkhtmltopdf:
//   indexTmpl  → corpo do RI (metadados + conteúdo + qualificações)
//   headerTmpl → cabeçalho repetido em toda página via --header-html
//   footerTmpl → rodapé repetido em toda página via --footer-html
//
// wkhtmltopdf renderiza --header-html e --footer-html no espaço das margens
// (--margin-top / --margin-bottom) de CADA página. É mais confiável que
// position:fixed no body.

const legalNotice = `&ldquo;O sigilo deste documento é protegido e controlado pela Lei nº 12.527/2011. A divulgação, a revelação, o fornecimento, a utilização ou a reprodução desautorizada de seu conteúdo, a qualquer tempo, meio ou modo, inclusive mediante acesso ou facilitação de acesso indevidos, constituem condutas ilícitas que ensejam responsabilidades penais, civis e administrativas.&rdquo;`

// indexTmpl — corpo do documento (sem header/footer; vêm dos outros HTMLs).
var indexTmpl = template.Must(template.New("ri").Parse(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8" />
<title>{{ .TitleLine }}</title>
<style>
body {
    margin: 0;
    /* As margens laterais do wkhtmltopdf foram zeradas pra dar largura total
       às áreas de header/footer (necessário pro ribbon ser full bleed).
       Padding aqui mantém o conteúdo principal recuado 20mm de cada lado. */
    padding: 0 20mm;
    font-family: Arial, Helvetica, sans-serif;
    font-size: 11pt;
    color: #000;
}

.data-table {
    width: 100%;
    border-collapse: collapse;
    margin: 0 0 4pt 0;
}
.data-table th {
    text-align: left;
    font-weight: 700;
    padding: 1.5pt 2pt 1.5pt 0;
    white-space: nowrap;
    width: 95pt;
    vertical-align: top;
    font-size: 11pt;
}
.data-table td {
    text-align: left;
    vertical-align: top;
    padding: 1.5pt 0;
    font-size: 11pt;
}
.data-table .sep {
    padding: 1.5pt 4pt 1.5pt 0;
    width: 6pt;
    font-weight: 700;
    vertical-align: top;
}
hr {
    border: 0;
    height: 1px;
    background-color: #000;
    margin: 4pt 0 8pt 0;
}

.content { text-align: justify; }
.content p { margin: 0 0 6pt 0; }
/* Parágrafos vazios (Enter+Enter no editor → <p></p>) colapsam pra altura
   zero no wkhtmltopdf, somindo a linha em branco intencional do autor. O nbsp
   força uma caixa de linha, preservando cada linha em branco. */
.content p:empty::before { content: "\00a0"; }
.content h2, .content h3 { font-weight: bold; margin: 8pt 0 4pt 0; }
.content h2 { font-size: 14pt; }
.content h3 { font-size: 12pt; }
.content ul, .content ol { margin: 0 0 6pt 0; padding-left: 22pt; }
.content table { border-collapse: collapse; width: 100%; margin: 6pt 0; }
.content table td, .content table th {
    border: 1px solid #000;
    padding: 3pt 6pt;
    vertical-align: top;
}
.content img { max-width: 100%; height: auto; }
.content blockquote {
    margin: 6pt 18pt;
    padding: 0;
    border: none;
    font-family: "Courier New", Courier, monospace;
    font-size: 9pt;
    font-style: italic;
}
.content blockquote p { margin: 0 0 4pt 0; }
.content blockquote p:first-child::before { content: "\201C"; }
.content blockquote p:last-child::after   { content: "\201D"; }

.page-break { page-break-before: always; }

.qualifications h2 { font-size: 14pt; margin: 4pt 0 8pt 0; }
.suspect-table {
    margin: 10pt auto 0 auto;
    width: 95%;
    border: 1px solid #000;
    border-collapse: collapse;
    page-break-inside: avoid;
}
.suspect-header {
    font-size: 14pt;
    border: 1px solid #000;
    padding: 4pt;
    text-align: center;
    font-weight: bold;
}
.suspect-image-cell {
    border: 1px solid #000;
    text-align: center;
    width: 3.5cm;
    vertical-align: middle;
    padding: 4pt;
}
.suspect-photo { width: 3cm; height: 4cm; object-fit: cover; }
.suspect-photo-placeholder {
    display: inline-block;
    width: 3cm;
    height: 4cm;
    background: #c8c8c8;
}
.suspect-table .label {
    font-weight: 700;
    padding: 4pt 6pt;
    border: 1px solid #000;
    width: 18%;
    white-space: nowrap;
    vertical-align: top;
}
.suspect-table .label2 {
    padding: 4pt 6pt;
    border: 1px solid #000;
    vertical-align: top;
    font-family: Arial, Helvetica, sans-serif;
    font-size: 10pt;
    text-transform: uppercase;
}
.end_doc { margin-top: 25pt; font-size: 14pt; text-align: center; }

.draft-watermark {
    position: fixed;
    top: 38%;
    left: 0;
    right: 0;
    text-align: center;
    transform: rotate(-30deg);
    font-size: 110pt;
    color: rgba(180, 0, 0, 0.10);
    font-weight: 800;
    letter-spacing: 0.2em;
    pointer-events: none;
}
</style>
</head>
<body>

{{ if .IsDraft }}<div class="draft-watermark">RASCUNHO</div>{{ end }}

<table class="data-table" cellpadding="0" cellspacing="0">
    <tr><th>DATA</th><td class="sep">:</td><td>{{ if .DocDateShort }}{{ .DocDateShort }}{{ else }}*.*.*{{ end }}</td></tr>
    <tr><th>ASSUNTO</th><td class="sep">:</td><td>{{ if .Subject }}{{ .Subject }}{{ else }}*.*.*{{ end }}</td></tr>
    <tr><th>ORIGEM</th><td class="sep">:</td><td>{{ if .Origin }}{{ .Origin }}{{ else }}*.*.*{{ end }}</td></tr>
    <tr><th>DIFUSÃO</th><td class="sep">:</td><td>{{ if .Diffusion }}{{ .Diffusion }}{{ else }}*.*.*{{ end }}</td></tr>
    <tr><th>DIF. ANT</th><td class="sep">:</td><td>{{ if .PriorDiffusion }}{{ .PriorDiffusion }}{{ else }}*.*.*{{ end }}</td></tr>
    <tr><th>REF</th><td class="sep">:</td><td>{{ if .Reference }}{{ .Reference }}{{ else }}*.*.*{{ end }}</td></tr>
    <tr><th>ANEXO(S)</th><td class="sep">:</td><td>{{ if .Attachments }}{{ .Attachments }}{{ else }}*.*.*{{ end }}</td></tr>
</table>
<hr>

<div class="content">{{ .BodyHTML }}</div>

{{ if .Qualifications }}
<div class="page-break"></div>

<div class="qualifications">
    <h2>QUALIFICAÇÕES</h2>
    {{ range .Qualifications }}
    <table class="suspect-table" cellpadding="0" cellspacing="0" align="center">
        <thead>
            <tr><th colspan="3" class="suspect-header">{{ .Name }}</th></tr>
        </thead>
        <tbody>
            <tr>
                <td rowspan="6" class="suspect-image-cell">
                    {{ if .PhotoData }}<img src="{{ .PhotoData }}" alt="Foto" class="suspect-photo" />{{ else }}<span class="suspect-photo-placeholder"></span>{{ end }}
                </td>
                {{ if .Fields }}
                {{ $first := index .Fields 0 }}
                <td class="label">{{ $first.K }}</td>
                <td class="label2">{{ if $first.V }}{{ $first.V }}{{ else }}*.*.*{{ end }}</td>
                {{ end }}
            </tr>
            {{ range $i, $f := .Fields }}{{ if gt $i 0 }}
            <tr>
                <td class="label">{{ $f.K }}</td>
                <td class="label2">{{ if $f.V }}{{ $f.V }}{{ else }}*.*.*{{ end }}</td>
            </tr>
            {{ end }}{{ end }}
        </tbody>
    </table>
    {{ end }}
</div>
{{ end }}

<div class="end_doc">*.*.*</div>

</body>
</html>`))

// headerTmpl — cabeçalho repetido em todas as páginas via --header-html do
// wkhtmltopdf. Mantém SECRETO + brasoes + caixa de título.
var headerTmpl = template.Must(template.New("hd").Parse(`<!DOCTYPE html>
<html><head><meta charset="UTF-8" />
<style>
html, body { margin: 0; padding: 0; }
body {
    /* Header canvas é 210mm (margins do wkhtmltopdf zeradas). Padding lateral
       traz o conteúdo do header pra dentro de 20mm em cada lado, mantendo
       coerência visual com o corpo. */
    padding: 6mm 20mm 0 20mm;
    font-family: Arial, Helvetica, sans-serif;
    color: #000;
    width: 100%;
    box-sizing: border-box;
}
.sigilo {
    font-size: 12pt;
    font-weight: bold;
    color: #ff0000;
    text-align: center;
    margin: 0 0 6pt 0;
    letter-spacing: 0.08em;
}
.logos { text-align: center; margin: 0 0 6pt 0; }
.logos table { width: 100%; border-collapse: collapse; border: none; table-layout: fixed; }
.logos td { vertical-align: middle; padding: 0 6pt; border: none; }
/* max-width: 100% impede que um logo largo transborde a célula (table-layout
   fixed) e "vaze" pra direita; object-fit preserva a proporção ao limitar. */
.logos img { display: inline-block; vertical-align: middle; max-height: 2cm; max-width: 100%; width: auto; object-fit: contain; }
/* Por padrão (todas as páginas exceto a primeira): apenas o texto, sem borda. */
.title {
    border: none;
    padding: 4pt 8pt;
    text-align: center;
    font-size: 11pt;
    font-weight: 700;
    color: #2f5496;
    width: 100%;
    box-sizing: border-box;
}
/* Página 1: borda preta envolvendo o texto. A classe first-page é adicionada
   via JS no <body> lendo o page number da query string do wkhtmltopdf. */
body.first-page .title {
    border: 1px solid #000;
}
/* Stamp forense invisível: ESCRITO EM BRANCO no canto superior esquerdo de
   TODA página (o header é renderizado uma vez por página pelo wkhtmltopdf).
   position:absolute pra não empurrar nenhum elemento; z-index negativo pra
   ficar atrás dos demais (qualquer overlap fica coberto). Visível só ao
   selecionar/copiar texto ou inspecionar o PDF — basta abrir no Acrobat e
   "selecionar tudo" pra revelar AGENTE + DATA do download. */
.hidden-stamp {
    position: absolute;
    top: 0;
    left: 0;
    font-size: 6pt;
    color: #ffffff;
    z-index: -1;
    white-space: nowrap;
    pointer-events: none;
}
/* Paginação no canto superior direito, flutuando sobre o conteúdo. */
.page-counter {
    position: absolute;
    top: 4mm;
    right: 8mm;
    font-size: 9pt;
    font-weight: 700;
    color: #808080;
    white-space: nowrap;
    z-index: 5;
}
</style>
<script type="text/javascript">
function applyFirstPage() {
    var params = {};
    var pairs = window.location.search.substring(1).split('&');
    for (var i = 0; i < pairs.length; i++) {
        var p = pairs[i].split('=', 2);
        params[p[0]] = decodeURIComponent(p[1] || '');
    }
    if (params['page'] === '1') {
        document.body.className = (document.body.className + ' first-page').trim();
    }
    // wkhtmltopdf passa "page" e "topage" na query string do header — preenche
    // os spans correspondentes manualmente (não há auto-inject por classe).
    function setText(cls, val) {
        var els = document.getElementsByClassName(cls);
        for (var i = 0; i < els.length; i++) els[i].textContent = val || '';
    }
    setText('page', params['page']);
    setText('topage', params['topage']);
    fitTitle();
}

// fitTitle reduz a fonte da barra de título só o necessário pra caber em uma
// única linha. A ORIGEM tem tamanho variável, então uma fonte fixa pode
// quebrar; nowrap garante que nunca quebre e o passo de 0.5pt encolhe até
// caber (ou até o mínimo, caso de origem absurdamente longa).
function fitTitle() {
    var el = document.getElementsByClassName('title')[0];
    if (!el) return;
    el.style.whiteSpace = 'nowrap';
    var size = 11; // casa com .title font-size no CSS
    var min = 7;
    el.style.fontSize = size + 'pt';
    while (el.scrollWidth > el.clientWidth && size > min) {
        size -= 0.5;
        el.style.fontSize = size + 'pt';
    }
}
</script>
</head><body onload="applyFirstPage()">
<div class="hidden-stamp">{{ .AgentCode }} {{ .GeneratedByName }} - {{ .GeneratedAtBR }}</div>
<div class="page-counter"><span class="page"></span>/<span class="topage"></span></div>
<div class="sigilo">{{ .Classification }}</div>
<div class="logos">
    <table>
        <tr>
            <td style="text-align: left; width: 18%;">
                {{ if .QRCodeData }}<img src="{{ .QRCodeData }}" alt="" />{{ end }}
            </td>
            <td style="text-align: center; width: 64%;">
                {{ if .BrasaoInstitucional }}<img src="{{ .BrasaoInstitucional }}" alt="" />{{ end }}
            </td>
            <td style="text-align: right; width: 18%;">
                {{ if .BrasaoSAIData }}<img src="{{ .BrasaoSAIData }}" alt="" />{{ end }}
            </td>
        </tr>
    </table>
</div>
<div class="title">{{ .TitleLine }}</div>
</body></html>`))

// footerTmpl — rodapé repetido em todas as páginas via --footer-html. Caixa
// vermelha do aviso + SECRETO + faixa ornamental colada nas bordas inferiores.
var footerTmpl = template.Must(template.New("ft").Parse(`<!DOCTYPE html>
<html><head><meta charset="UTF-8" />
<style>
html, body { margin: 0; padding: 0; }
body {
    font-family: Arial, Helvetica, sans-serif;
    color: #000;
    width: 100%;
    /* padding-top empurra o conteúdo do rodapé (warningborder + sigilo) pra
       baixo dentro do canvas de 50mm (--margin-bottom no wkhtmltopdf),
       criando ~10mm de respiro entre a última linha do corpo e a caixa do
       aviso. Usamos padding em vez de margin pra evitar margin-collapse
       com o primeiro filho (o margin-top colapsaria pra cima e sumiria). */
    padding-top: 10mm;
}
/* .safe é o wrapper centralizado pro conteúdo "normal" do rodapé (caixa do
   aviso e SECRETO). Como body é 210mm (margens do wkhtmltopdf zeradas),
   aplicamos margin 0 20mm pra manter recuo de 20mm cada lado. */
.safe { margin: 0 20mm; }

.warningborder {
    border: 1px solid #ff0000;
    padding: 2pt 4pt;
    color: #ff0000;
    font-size: 8pt;
    font-weight: 700;
    text-align: justify;
    line-height: 1.25;
    box-sizing: border-box;
    width: 100%;
}
.warningborder p { margin: 0; }
.message2 { text-align: center; margin-top: 2pt; }
.sigilo {
    font-size: 12pt;
    font-weight: bold;
    color: #ff0000;
    text-align: center;
    margin: 6pt 0 4pt 0;
    letter-spacing: 0.08em;
}
/* Ribbon FORA do .safe → ocupa os 210mm completos do body (canvas inteiro
   da A4) sem nenhuma margem lateral. */
.footermark {
    width: 100%;
    margin: 0;
    padding: 0;
    line-height: 0;
}
.footermark img { width: 100%; height: auto; display: block; }
</style></head><body>
<div class="safe">
    <div class="warningborder">
        <p>` + legalNotice + `</p>
        <p class="message2">É vedada a retransmissão deste documento</p>
    </div>
    <div class="sigilo">{{ .Classification }}</div>
</div>
{{ if .FooterRibbonData }}<div class="footermark"><img src="{{ .FooterRibbonData }}" alt="" /></div>{{ end }}
</body></html>`))

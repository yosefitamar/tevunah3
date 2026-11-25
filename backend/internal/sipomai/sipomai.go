// Package sipomai pede à IA sugestões para as pendências que travam o envio
// de uma ocorrência ao SIPOM: natureza, bairro do catálogo, sexo dos
// envolvidos e a tradução das apreensões (arma, droga, veículo).
//
// A IA nunca grava nada. Ela escolhe entre as opções do catálogo que este
// pacote lhe entrega; Parse confere cada id contra o catálogo e devolve
// sugestões que o analista confirma — e a confirmação passa pelos mesmos
// endpoints da correção manual (validação, auditoria e termos aprendidos).
package sipomai

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Feature é o nome no registro de uso da IA (ai_usage).
const Feature = "sipom_pendencias"

// Tipos de sugestão — cada um corresponde a um endpoint de correção.
const (
	KindNatureza     = "natureza"      // PUT .../sipom/natureza
	KindBairro       = "bairro"        // PUT .../sipom/bairro
	KindSexo         = "sexo"          // PUT .../sipom/pessoas/{pid}/sexo
	KindArma         = "arma"          // PUT .../sipom/materiais/armas/{pos}
	KindDroga        = "droga"         // PUT .../sipom/materiais/drogas/{pos}
	KindVeiculo      = "veiculo"       // PUT .../sipom/materiais/veiculos/{pos}
	KindVeiculoPlaca = "veiculo_placa" // PUT .../sipom/veiculos/{pos}/placa
	// KindVeiculoOutros: placa não identificada — o veículo vai ao SIPOM como
	// material "Outros", com a descrição (PUT .../veiculos/{pos}/placa com
	// sem_placa e descricao).
	KindVeiculoOutros = "veiculo_outros"
	// KindRemover: o item de apreensão não deve ir ao SIPOM (duplicado, não
	// apreendido, ou conforme orientação) — PUT .../materiais/{kind}/{pos}/remocao.
	KindRemover = "remover"
	// KindAdicionar: o histórico cita um objeto apreendido que não está na
	// lista — POST .../materiais/{kind}.
	KindAdicionar = "adicionar"
	// Da recusa do SIPOM: o ano do veículo (PUT .../veiculos/{pos}/ano), o
	// veículo com placa como "Outros" (PUT .../veiculos/{pos}/outros) e a
	// explicação do que a IA não resolve (só informa).
	KindVeiculoAno        = "veiculo_ano"
	KindVeiculoComoOutros = "veiculo_como_outros"
	KindRecusaInfo        = "recusa_info"
)

// Codes são as pendências que a IA ajuda a resolver.
var Codes = []string{
	sipom.PendNatureza, sipom.PendNatConfirm, sipom.PendBairroCatalogo,
	sipom.PendPessoaSexo, sipom.PendPessoaSexoConfirm,
	sipom.PendArma, sipom.PendDroga, sipom.PendVeiculo, sipom.PendVeiculoPlaca,
	sipom.PendEnvioRecusado,
}

// Handles diz se a IA tem o que fazer com as pendências da ocorrência.
func Handles(pending []string) bool {
	for _, p := range pending {
		for _, c := range Codes {
			if p == c {
				return true
			}
		}
	}
	return false
}

// Suggestion é uma sugestão pronta para a tela: o que muda, em texto, e o
// corpo do endpoint de correção (Apply) quando dá para aplicar.
type Suggestion struct {
	Kind string `json:"kind"`
	// Code é a pendência que a sugestão resolve.
	Code string `json:"code"`
	// Target diz a que se refere ("Envolvido: FULANO", "Droga 1: …").
	Target   string `json:"target"`
	PersonID string `json:"person_id,omitempty"`
	Index    *int   `json:"index,omitempty"`
	// Value é a sugestão por extenso; Current, o que está na ficha hoje
	// (a sugestão do de-para ou do censo, quando houver).
	Value   string `json:"value"`
	Current string `json:"current,omitempty"`
	// Agrees: a IA concorda com a sugestão automática que já estava na ficha.
	Agrees    bool   `json:"agrees,omitempty"`
	Confianca string `json:"confianca"` // alta | media | baixa
	Motivo    string `json:"motivo"`
	// Applicable: a sugestão é completa e válida no catálogo. Sem isso ela
	// só informa (a IA não achou opção que sirva).
	Applicable bool           `json:"applicable"`
	Apply      map[string]any `json:"apply,omitempty"`
	// Guidance: a orientação de analista que a IA diz ter seguido.
	Guidance *GuidanceRef `json:"guidance,omitempty"`
}

// Guidance é uma orientação de analista que vale para o pedido (aprovada,
// ou pendente do próprio usuário).
type Guidance struct {
	ID       string
	Topic    string // natureza | bairro | sexo | apreensoes | geral
	CidadeID *int
	Body     string
	Author   string
	Date     string // dd/mm/aaaa
	Pending  bool   // ainda não aprovada: vale só para o autor
}

// GuidanceRef é a orientação citada numa sugestão.
type GuidanceRef struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Pending bool   `json:"pending"`
}

// sectionTopic diz o tema de orientação de cada seção do pedido.
var sectionTopic = map[string]string{
	"natureza": "natureza", "bairro": "bairro", "pessoas": "sexo",
	"armas": "apreensoes", "drogas": "apreensoes", "veiculos": "apreensoes", "apreensoes_ficha": "apreensoes",
	"recusa": "recusa",
}

// Request é o caso montado para a IA e o que Parse precisa para conferir a
// resposta.
type Request struct {
	Prompt string
	// Sections diz o que foi pedido (natureza, bairro, pessoas, armas,
	// drogas, veiculos).
	Sections []string

	so       *opsreport.StoredOccurrence
	cat      *sipom.Catalog
	natCode  string // natureza | natureza_confirmar | ""
	cidadeID int
	people   map[string]personRef // "P1" → envolvido
	weapons  []int                // posições pedidas
	drugs    []int
	vehicles map[int]vehicleAsk
	guidance map[string]Guidance // "G1" → orientação
	// AddPrompt é o pedido da segunda chamada (apreensões que faltam).
	AddPrompt string
	// RefusalPrompt é o pedido da terceira chamada: a recusa do SIPOM no
	// último envio (vazio = a ficha não foi recusada).
	RefusalPrompt string
	refusalRefs   []opsreport.MaterialRef
	guidanceOrder []string // referências G1… na ordem
	head          string   // ocorrência e histórico, comum aos dois pedidos
	seizureList   string
	// CidadeID é a cidade da ocorrência no catálogo (0 = não resolvida).
	CidadeID int
}

// cite devolve a orientação que a IA disse ter seguido, se ela existe no
// pedido.
func (req *Request) cite(m meta) *GuidanceRef {
	g, ok := req.guidance[strings.TrimSpace(m.Orientacao)]
	if !ok {
		return nil
	}
	return &GuidanceRef{ID: g.ID, Body: g.Body, Author: g.Author, Date: g.Date, Pending: g.Pending}
}

// UsedGuidance são os ids das orientações citadas nas sugestões.
func UsedGuidance(list []Suggestion) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range list {
		if s.Guidance != nil && !seen[s.Guidance.ID] {
			seen[s.Guidance.ID] = true
			out = append(out, s.Guidance.ID)
		}
	}
	return out
}

type personRef struct {
	id, name   string
	suggestion int
	code       string
}

type vehicleAsk struct {
	material   bool // tipo/cor/marca-modelo/situação
	plate      bool // placa ausente
	candidates map[int]bool
}

// Build monta o pedido; ok = false quando nenhuma pendência da ocorrência é
// das que a IA resolve.
func Build(cat *sipom.Catalog, terms *sipom.TermMap, so *opsreport.StoredOccurrence, guidance []Guidance) (*Request, bool) {
	pend := map[string]bool{}
	for _, p := range so.Sipom.Pending {
		pend[p] = true
	}
	req := &Request{so: so, cat: cat, people: map[string]personRef{}, vehicles: map[int]vehicleAsk{},
		guidance: map[string]Guidance{}}
	if so.Sipom.CidadeID != nil {
		req.CidadeID = *so.Sipom.CidadeID
	}
	req.assignGuidance(guidance)
	var b strings.Builder
	o := &so.Occurrence

	b.WriteString("# OCORRÊNCIA\n")
	fmt.Fprintf(&b, "Data: %s %s\n", o.OccurredOn.Format("02/01/2006"), o.StartTime)
	fmt.Fprintf(&b, "Naturezas no relatório: %s\n", strings.Join(o.Natures, "; "))
	fmt.Fprintf(&b, "Local: %s — bairro %q — %s\n", o.PlaceAddress, o.PlaceNeighborhood, o.PlaceCity)
	if strings.TrimSpace(o.ApproachAddress+o.ApproachNeighborhood) != "" {
		fmt.Fprintf(&b, "Local da abordagem: %s — bairro %q — %s\n", o.ApproachAddress, o.ApproachNeighborhood, o.ApproachCity)
	}
	if t := strings.TrimSpace(o.SeizedObjects); t != "" {
		fmt.Fprintf(&b, "Objetos apreendidos (texto do relatório): %s\n", t)
	}
	fmt.Fprintf(&b, "\n## HISTÓRICO\n%s\n", strings.TrimSpace(o.Narrative))
	req.head = b.String()

	// ── Natureza ──
	switch {
	case pend[sipom.PendNatureza]:
		req.natCode = sipom.PendNatureza
	case pend[sipom.PendNatConfirm]:
		req.natCode = sipom.PendNatConfirm
	}
	if req.natCode != "" {
		req.Sections = append(req.Sections, "natureza")
		b.WriteString("\n# NATUREZA — escolha a natureza do SIPOM que corresponde ao fato principal\n")
		if req.natCode == sipom.PendNatConfirm && so.Sipom.NaturezaID != nil {
			if n, ok := cat.Natureza(*so.Sipom.NaturezaID); ok {
				fmt.Fprintf(&b, "Sugestão automática atual (de-para): %d | %s — confirme mantendo este id ou troque.\n", n.ID, n.Nome)
			}
		}
		b.WriteString("Naturezas do SIPOM (id | nome):\n")
		for _, n := range cat.Naturezas() {
			fmt.Fprintf(&b, "%d | %s\n", n.ID, n.Nome)
		}
	}

	// ── Bairro ──
	if pend[sipom.PendBairroCatalogo] && so.Sipom.CidadeID != nil {
		req.cidadeID = *so.Sipom.CidadeID
		req.Sections = append(req.Sections, "bairro")
		fmt.Fprintf(&b, "\n# BAIRRO — o bairro %q não está no catálogo do SIPOM para %s.\n", o.PlaceNeighborhood, cat.CidadeNome(req.cidadeID))
		b.WriteString("Escolha o bairro da lista que contém o local (pelo nome parecido, pelo endereço ou por ser o bairro de que o loteamento/conjunto/localidade faz parte). Se nenhum servir com segurança, bairro_id null.\n")
		if o.Geo.Located() {
			fmt.Fprintf(&b, "Coordenada do local: %.5f, %.5f\n", *o.Geo.Lat, *o.Geo.Lng)
		}
		b.WriteString("Bairros da cidade (id | nome):\n")
		for _, x := range cat.Bairros(req.cidadeID) {
			fmt.Fprintf(&b, "%d | %s\n", x.ID, x.Nome)
		}
	}

	// ── Sexo dos envolvidos ──
	if pend[sipom.PendPessoaSexo] || pend[sipom.PendPessoaSexoConfirm] {
		report := map[string]opsreport.StoredPerson{}
		for _, p := range so.People {
			report[p.ID] = p
		}
		var lines []string
		for _, p := range opsreport.SipomPeople(so) {
			if p.Sexo != 0 || p.Nome == "" || p.ID == "" {
				continue
			}
			ref := "P" + strconv.Itoa(len(req.people)+1)
			rp := report[p.ID]
			pr := personRef{id: p.ID, name: p.Nome, suggestion: p.SexoSugerido, code: sipom.PendPessoaSexo}
			line := fmt.Sprintf("%s | papel: %s | nome: %s", ref, rp.Role, p.Nome)
			if rp.Age != nil {
				line += fmt.Sprintf(" | idade: %d", *rp.Age)
			}
			if rp.Note != "" {
				line += " | anotação: " + rp.Note
			}
			if p.SexoSugerido != 0 {
				pr.code = sipom.PendPessoaSexoConfirm
				line += " | sugestão automática: " + sexoNome(p.SexoSugerido) + " (" + p.SexoFonte + ")"
			}
			req.people[ref] = pr
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			req.Sections = append(req.Sections, "pessoas")
			b.WriteString("\n# SEXO DOS ENVOLVIDOS — diga o sexo de cada um (M ou F). Use a concordância do histórico (\"a acusada\", \"o mesmo\", \"a nacional\", \"ele/ela\") e o nome. Sem base, \"indefinido\".\n")
			b.WriteString(strings.Join(lines, "\n") + "\n")
		}
	}

	// ── Armas ──
	if pend[sipom.PendArma] {
		var lines []string
		for i, w := range o.Weapons {
			if w.SipomTipoID != nil || w.Removed() {
				continue
			}
			req.weapons = append(req.weapons, i)
			lines = append(lines, fmt.Sprintf("item %d | tipo: %s | marca: %s | modelo: %s | calibre: %s",
				i, w.Kind, w.Brand, w.Model, w.Caliber))
		}
		if len(lines) > 0 {
			req.Sections = append(req.Sections, "armas")
			b.WriteString("\n# ARMAS — traduza cada arma para os ids do SIPOM (tipo obrigatório; marca e calibre só se o texto disser)\n")
			b.WriteString(strings.Join(lines, "\n") + "\n")
			writeRefs(&b, "Tipos de arma", cat.ArmaTipos())
			writeRefs(&b, "Marcas de arma", cat.ArmaMarcas())
			writeRefs(&b, "Calibres", cat.ArmaCalibres())
		}
	}

	// ── Drogas ──
	if pend[sipom.PendDroga] {
		var lines []string
		for i, d := range o.Drugs {
			if (d.SipomDrogaID != nil && d.SipomQuantidade != nil) || d.Removed() {
				continue
			}
			req.drugs = append(req.drugs, i)
			line := fmt.Sprintf("item %d | descrição: %s", i, d.Description)
			if d.Grams != nil {
				line += fmt.Sprintf(" | gramas: %g", *d.Grams)
			}
			if d.Packages != nil {
				line += fmt.Sprintf(" | embalagens/porções: %d", *d.Packages)
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			req.Sections = append(req.Sections, "drogas")
			b.WriteString("\n# DROGAS — escolha a droga do SIPOM e a quantidade na UNIDADE dela (número inteiro; arredonde). Quantidade que o texto não dá: null.\n")
			b.WriteString(strings.Join(lines, "\n") + "\n")
			b.WriteString("Drogas do SIPOM (id | nome | unidade):\n")
			for _, d := range cat.Drogas() {
				fmt.Fprintf(&b, "%d | %s | %s\n", d.ID, d.Nome, d.Unidade)
			}
		}
	}

	// ── Veículos ──
	if pend[sipom.PendVeiculo] || pend[sipom.PendVeiculoPlaca] {
		var lines []string
		for i, v := range o.Vehicles {
			if v.Removed() {
				continue
			}
			ask := vehicleAsk{
				material: v.SipomSituacao == 0 || !opsreport.VehicleComplete(v),
				plate:    plateKey(v.Plate) == "" && !opsreport.VehicleAsOutros(v),
			}
			// Sem placa e já indo como "Outros" (ciclomotor ou falta
			// confirmada), o resto do veículo não importa.
			if plateKey(v.Plate) == "" && !ask.plate {
				continue
			}
			if !ask.material && !ask.plate {
				continue
			}
			line := fmt.Sprintf("item %d | tipo: %s | marca: %s | modelo: %s | cor: %s | placa: %s",
				i, v.Kind, v.Brand, v.Model, v.Color, orDash(v.Plate))
			if ask.plate {
				line += "\n  SEM PLACA NO RELATÓRIO: se a placa estiver escrita no histórico ou nos objetos, informe-a em placa. Se não estiver, placa null e escreva outros_descricao — o veículo vai ao SIPOM como material OUTROS."
			}
			if ask.material {
				ask.candidates = map[int]bool{}
				var cands []string
				for _, m := range vehicleCandidates(cat, terms, v) {
					ask.candidates[m.Codigo] = true
					cands = append(cands, fmt.Sprintf("%d | %s", m.Codigo, m.Descricao))
				}
				if len(cands) > 0 {
					line += "\n  marca/modelo candidatos (código | descrição):\n  " + strings.Join(cands, "\n  ")
				} else {
					line += "\n  (nenhuma marca/modelo candidata no catálogo: marca_modelo_codigo null)"
				}
			}
			req.vehicles[i] = ask
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			req.Sections = append(req.Sections, "veiculos")
			b.WriteString("\n# VEÍCULOS — situação (apreendido ou recuperado: recuperado é veículo roubado/furtado encontrado), tipo, cor e marca/modelo do SIPOM; placa só se aparecer escrita no texto (nunca invente). outros_descricao (só para veículo sem placa): descrição objetiva em até 300 caracteres — tipo, marca, modelo, cor, chassi/motor se citados, estado (avarias, adulteração) e \"sem placa identificada\" — só o que o relatório diz, sem mencionar o que ele não informa; para os demais, texto vazio.\n")
			if o.RecoveredVehicle() {
				b.WriteString("Uma das naturezas fala em recuperação de veículo.\n")
			}
			b.WriteString(strings.Join(lines, "\n") + "\n")
			writeRefs(&b, "Tipos de veículo (código | nome)", cat.VeiculoTipos())
			writeRefs(&b, "Cores (código | nome)", cat.VeiculoCores())
		}
	}

	if pend[sipom.PendEnvioRecusado] && strings.TrimSpace(so.Sipom.Send.Erro) != "" {
		req.RefusalPrompt = req.buildRefusalPrompt()
	}
	if len(req.Sections) == 0 {
		if req.RefusalPrompt == "" {
			return nil, false
		}
		return req, true // só a recusa: sem pedido principal
	}
	writeRemoval(&b, req)
	topics := map[string]bool{"geral": true}
	for _, s := range req.Sections {
		topics[sectionTopic[s]] = true
	}
	req.writeGuidance(&b, topics, "Ao seguir uma, ponha a referência dela (ex.: \"G1\") em orientacao; senão, texto vazio.")
	fmt.Fprintf(&b, "\nResponda só as seções pedidas: %s. As demais: null ou lista vazia.\n", strings.Join(req.Sections, ", "))
	req.Prompt = b.String()
	req.AddPrompt = req.buildAddPrompt()
	return req, true
}

// buildRefusalPrompt monta o pedido da recusa: a mensagem do SIPOM, a que
// item da ficha cada "materiais.N" se refere, e as ações possíveis.
func (req *Request) buildRefusalPrompt() string {
	o := &req.so.Occurrence
	var b strings.Builder
	b.WriteString(req.head)
	fmt.Fprintf(&b, "\n# RECUSA DO SIPOM NO ÚLTIMO ENVIO\nMensagem: %s\n", strings.TrimSpace(req.so.Sipom.Send.Erro))
	req.refusalRefs = opsreport.PayloadMaterialRefs(req.so)
	if len(req.refusalRefs) > 0 {
		b.WriteString("\nMateriais do envio (\"materiais.N\" na mensagem = a linha N):\n")
		for n, m := range req.refusalRefs {
			switch m.Kind {
			case "armas":
				w := o.Weapons[m.Index]
				fmt.Fprintf(&b, "materiais.%d = arma %d | %s %s %s %s\n", n, m.Index, w.Kind, w.Brand, w.Model, w.Caliber)
			case "drogas":
				fmt.Fprintf(&b, "materiais.%d = droga %d | %s\n", n, m.Index, o.Drugs[m.Index].Description)
			case "veiculos":
				v := o.Vehicles[m.Index]
				fmt.Fprintf(&b, "materiais.%d = veiculo %d | %s %s %s %s | placa %s | ano fabricação %s | ano modelo %s\n",
					n, m.Index, v.Kind, v.Brand, v.Model, v.Color, orDash(v.Plate), yearOrDash(v.SipomAnoFabricacao), yearOrDash(v.SipomAnoModelo))
			}
		}
	}
	b.WriteString(`
# O QUE FAZER — proponha em acoes a correção de cada item recusado, só com estas ações:
- "ano": ano_fabricacao (e ano_modelo, se souber; senão 0) do veículo. SÓ se o ano estiver escrito no relatório ou vier de uma orientação dos analistas — nunca estime.
- "outros": mandar o veículo como material OUTROS (sem cadastro de veículo no SIPOM), com descricao objetiva (tipo, marca, modelo, cor, placa). Use quando uma orientação mandar, ou quando o dado exigido (ano) não existe no relatório e uma orientação permitir.
- "remover": tirar o item da ocorrência (lançado por engano, não apreendido).
Em material use arma | droga | veiculo e em item o número do item na ficha (o que vem depois de "veiculo", "arma" ou "droga" na lista acima, não o N de materiais.N).
Em explicacao, diga em uma ou duas frases o que a recusa significa e o que falta para resolver — sobretudo quando não há ação possível (ex.: OPM não permitida para o cliente é configuração do SIPOM; ano que o relatório não traz depende do analista).
`)
	req.writeGuidance(&b, map[string]bool{"recusa": true, "apreensoes": true, "geral": true},
		"Ao seguir uma, ponha a referência dela (ex.: \"G1\") em orientacao da ação; senão, texto vazio.")
	return b.String()
}

func yearOrDash(y *int) string {
	if y == nil {
		return "—"
	}
	return strconv.Itoa(*y)
}

// writeRemoval lista as apreensões ativas da ficha para a IA apontar as que
// não devem ir ao SIPOM. Só entra junto de outra seção: sozinha, não motiva
// a consulta.
func writeRemoval(b *strings.Builder, req *Request) {
	o := &req.so.Occurrence
	var lines []string
	for i, w := range o.Weapons {
		if !w.Removed() {
			lines = append(lines, fmt.Sprintf("arma %d | %s %s %s %s nº %s", i, w.Kind, w.Brand, w.Model, w.Caliber, orDash(w.Serial)))
		}
	}
	for i, d := range o.Drugs {
		if !d.Removed() {
			line := fmt.Sprintf("droga %d | %s", i, d.Description)
			if d.Grams != nil {
				line += fmt.Sprintf(" | %g g", *d.Grams)
			}
			lines = append(lines, line)
		}
	}
	for i, v := range o.Vehicles {
		if !v.Removed() {
			lines = append(lines, fmt.Sprintf("veiculo %d | %s %s %s %s placa %s", i, v.Kind, v.Brand, v.Model, v.Color, orDash(v.Plate)))
		}
	}
	req.Sections = append(req.Sections, "apreensoes_ficha")
	if len(lines) == 0 {
		lines = append(lines, "(nenhuma)")
	}
	req.seizureList = strings.Join(lines, "\n")
	b.WriteString("\n# APREENSÕES DA FICHA — em remover, aponte SÓ itens que não devem ir ao SIPOM: lixo de leitura do PDF (data, hora, endereço de site ou texto que não é um objeto), o mesmo objeto lançado duas vezes, item que o histórico diz que não foi apreendido (devolvido ao dono, apenas citado), ou o que uma orientação dos analistas mandar remover. Na dúvida, não remova (lista vazia).\n")
	b.WriteString(strings.Join(lines, "\n") + "\n")
}

// buildAddPrompt monta o pedido das apreensões que faltam: a ocorrência, a
// lista atual e as orientações de apreensões e gerais (com as mesmas
// referências G1… do pedido principal).
func (req *Request) buildAddPrompt() string {
	var b strings.Builder
	b.WriteString(req.head)
	b.WriteString("\n# APREENSÕES JÁ NA FICHA\n" + req.seizureList + "\n")
	b.WriteString("\n# O QUE FALTA — em adicionar, inclua SÓ arma, droga ou veículo que o histórico ou os objetos apreendidos dizem claramente que FOI APREENDIDO e que NÃO está na lista acima (nunca repita um item da lista, mesmo com outra grafia; munição, celular, dinheiro e outros objetos não entram). Peso em gramas e porções só se o texto disser (senão null e 0). Campos que não se aplicam ao material: texto vazio. Na dúvida, lista vazia.\n")
	req.writeGuidance(&b, map[string]bool{"apreensoes": true, "geral": true},
		"Ao seguir uma, ponha a referência em orientacao (senão, texto vazio).")
	return b.String()
}

// maxGuidance limita as orientações por pedido.
const maxGuidance = 40

// assignGuidance dá as referências G1… às orientações que valem para a
// cidade da ocorrência — as da cidade antes das de todas as cidades. As
// referências são as mesmas nos três pedidos.
func (req *Request) assignGuidance(all []Guidance) {
	var local, global []Guidance
	for _, g := range all {
		switch {
		case g.CidadeID == nil:
			global = append(global, g)
		case *g.CidadeID == req.CidadeID:
			local = append(local, g)
		}
	}
	list := append(local, global...)
	if len(list) > maxGuidance {
		list = list[:maxGuidance]
	}
	for i, g := range list {
		ref := "G" + strconv.Itoa(i+1)
		req.guidance[ref] = g
		req.guidanceOrder = append(req.guidanceOrder, ref)
	}
}

// writeGuidance põe no pedido as orientações dos temas indicados.
func (req *Request) writeGuidance(b *strings.Builder, topics map[string]bool, cite string) {
	var lines []string
	for _, ref := range req.guidanceOrder {
		g := req.guidance[ref]
		if !topics[g.Topic] {
			continue
		}
		scope := g.Topic
		if g.CidadeID != nil {
			scope += " · " + req.cat.CidadeNome(*g.CidadeID)
		}
		lines = append(lines, fmt.Sprintf("%s [%s] %s", ref, scope, g.Body))
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("\n# ORIENTAÇÕES DOS ANALISTAS — siga-as quando o caso se encaixar (valem mais que as regras gerais), mas continue escolhendo só opções das listas. " + cite + "\n")
	b.WriteString(strings.Join(lines, "\n") + "\n")
}

func writeRefs(b *strings.Builder, title string, refs []sipom.Ref) {
	fmt.Fprintf(b, "%s:\n", title)
	for _, r := range refs {
		fmt.Fprintf(b, "%d | %s\n", r.ID, r.Nome)
	}
}

// vehicleCandidates junta as linhas DENATRAN que podem ser o veículo: as do
// resolvedor automático e as da busca por marca + modelo e só pelo modelo.
func vehicleCandidates(cat *sipom.Catalog, terms *sipom.TermMap, v opsreport.Vehicle) []sipom.MarcaModelo {
	seen := map[int]bool{}
	var out []sipom.MarcaModelo
	add := func(ms []sipom.MarcaModelo) {
		for _, m := range ms {
			if len(out) >= 40 {
				return
			}
			if !seen[m.Codigo] {
				seen[m.Codigo] = true
				out = append(out, m)
			}
		}
	}
	if v.SipomMarcaModeloCodigo != nil {
		if m, ok := cat.MarcaModelo(*v.SipomMarcaModeloCodigo); ok {
			add([]sipom.MarcaModelo{m})
		}
	}
	add(cat.ResolveVehicle(sipom.VehicleInput{Kind: v.Kind, Brand: v.Brand, Model: v.Model, Color: v.Color}, terms).MarcaModeloCandidates)
	if q := strings.TrimSpace(v.Brand + " " + v.Model); q != "" {
		add(cat.SuggestMarcasModelos(q, 20))
	}
	if q := strings.TrimSpace(v.Model); q != "" {
		add(cat.SuggestMarcasModelos(q, 20))
	}
	return out
}

// ─── Instruções e schema ───

// System são as instruções fixas (vão em cache).
const System = `Você ajuda analistas de inteligência da Polícia Militar do Ceará a completar ocorrências antes do envio ao SIPOM (sistema da PM). Você recebe a ocorrência como veio do relatório operacional e, para cada pendência, a lista fechada de opções do catálogo do SIPOM.

Regras:
- Escolha SOMENTE ids/códigos presentes nas listas fornecidas. Se nenhuma opção servir com segurança, use null — o analista resolve à mão.
- Não invente fatos: placa, quantidade, marca e calibre só quando estiverem no texto.
- confianca: "alta" quando o texto diz explicitamente; "media" quando é a leitura mais provável; "baixa" quando é palpite.
- motivo: uma frase curta em português, citando o trecho do relatório que sustenta a escolha.
- Natureza: a do fato principal que gerou a ocorrência (a primeira natureza do relatório costuma ser a principal; o histórico desempata).
- Sexo: concordância de gênero no histórico ("a acusada", "a mesma", "o conduzido") vale mais que o nome.
- Drogas: quantidade inteira na unidade da droga escolhida (gramas → g; pedras, comprimidos, porções → unidades quando a unidade for unidade).
- Veículo: marca/modelo só entre os candidatos; se o texto não diz a versão (cilindrada, motor), escolha a mais comum e use no máximo confianca "media".
- Veículo: "recuperado" só para veículo com restrição de roubo/furto encontrado; veículo do autor usado no crime ou sem documentação é "apreendido".`

func nullable(t string) map[string]any {
	return map[string]any{"anyOf": []any{map[string]any{"type": t}, map[string]any{"type": "null"}}}
}

func obj(props map[string]any) map[string]any {
	req := make([]string, 0, len(props))
	for k := range props {
		req = append(req, k)
	}
	return map[string]any{"type": "object", "properties": props, "required": req, "additionalProperties": false}
}

func arr(item map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": item}
}

var confianca = map[string]any{"type": "string", "enum": []string{"alta", "media", "baixa"}}
var motivo = map[string]any{"type": "string"}

// Schema é o formato da resposta (structured outputs). Fixo: o schema
// compilado fica em cache na Anthropic.
var Schema = obj(map[string]any{
	"remover": arr(obj(map[string]any{
		"material":  map[string]any{"type": "string", "enum": []string{"arma", "droga", "veiculo"}},
		"item":      map[string]any{"type": "integer"},
		"confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
	"natureza": map[string]any{"anyOf": []any{
		obj(map[string]any{"natureza_id": nullable("integer"), "confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"}}),
		map[string]any{"type": "null"},
	}},
	"bairro": map[string]any{"anyOf": []any{
		obj(map[string]any{"bairro_id": nullable("integer"), "confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"}}),
		map[string]any{"type": "null"},
	}},
	"pessoas": arr(obj(map[string]any{
		"ref":       map[string]any{"type": "string"},
		"sexo":      map[string]any{"type": "string", "enum": []string{"M", "F", "indefinido"}},
		"confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
	"armas": arr(obj(map[string]any{
		"item": map[string]any{"type": "integer"}, "tipo_id": nullable("integer"),
		"marca_id": nullable("integer"), "calibre_id": nullable("integer"),
		"confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
	"drogas": arr(obj(map[string]any{
		"item": map[string]any{"type": "integer"}, "droga_id": nullable("integer"),
		"quantidade": nullable("number"), "confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
	"veiculos": arr(obj(map[string]any{
		"item":                map[string]any{"type": "integer"},
		"situacao":            map[string]any{"type": "string", "enum": []string{"apreendido", "recuperado", "indefinida"}},
		"tipo_codigo":         nullable("integer"),
		"cor_codigo":          nullable("integer"),
		"marca_modelo_codigo": nullable("integer"),
		"placa":               nullable("string"),
		"outros_descricao":    map[string]any{"type": "string"},
		"confianca":           confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
})

// AddSchema é o formato da segunda chamada, a das apreensões que faltam na
// ficha. Separada porque, junto da principal, o schema passa do tamanho que o
// structured outputs aceita ("compiled grammar is too large").
var AddSchema = obj(map[string]any{
	"adicionar": arr(obj(map[string]any{
		"material":  map[string]any{"type": "string", "enum": []string{"arma", "droga", "veiculo"}},
		"tipo":      map[string]any{"type": "string"},
		"marca":     map[string]any{"type": "string"},
		"modelo":    map[string]any{"type": "string"},
		"calibre":   map[string]any{"type": "string"},
		"numero":    map[string]any{"type": "string"},
		"descricao": map[string]any{"type": "string"},
		"gramas":    nullable("number"),
		"porcoes":   map[string]any{"type": "integer"},
		"placa":     map[string]any{"type": "string"},
		"cor":       map[string]any{"type": "string"},
		"confianca": confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
})

// RefusalSchema é o formato da terceira chamada (recusa do SIPOM).
var RefusalSchema = obj(map[string]any{
	"explicacao": map[string]any{"type": "string"},
	"acoes": arr(obj(map[string]any{
		"acao":           map[string]any{"type": "string", "enum": []string{"ano", "outros", "remover"}},
		"material":       map[string]any{"type": "string", "enum": []string{"arma", "droga", "veiculo"}},
		"item":           map[string]any{"type": "integer"},
		"ano_fabricacao": map[string]any{"type": "integer"},
		"ano_modelo":     map[string]any{"type": "integer"},
		"descricao":      map[string]any{"type": "string"},
		"confianca":      confianca, "motivo": motivo, "orientacao": map[string]any{"type": "string"},
	})),
})

type refusalAnswer struct {
	Explicacao string `json:"explicacao"`
	Acoes      []struct {
		Acao          string `json:"acao"`
		Material      string `json:"material"`
		Item          int    `json:"item"`
		AnoFabricacao int    `json:"ano_fabricacao"`
		AnoModelo     int    `json:"ano_modelo"`
		Descricao     string `json:"descricao"`
		meta
	} `json:"acoes"`
}

// ParseRefusal confere a resposta da recusa. O ano só passa se estiver no
// relatório ou vier de uma orientação citada (a IA não pode estimar).
func (req *Request) ParseRefusal(raw []byte) ([]Suggestion, error) {
	var a refusalAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	o := &req.so.Occurrence
	text := o.Narrative + " " + o.SeizedObjects
	out := []Suggestion{}
	if e := strings.TrimSpace(a.Explicacao); e != "" {
		out = append(out, Suggestion{Kind: KindRecusaInfo, Code: sipom.PendEnvioRecusado, Target: "Recusa do SIPOM",
			Value: e, Confianca: "media", Motivo: strings.TrimSpace(req.so.Sipom.Send.Erro)})
	}
	seen := map[string]bool{}
	for _, x := range a.Acoes {
		// Marca como vista só a ação aceita: uma inválida (ano estimado) não
		// pode barrar a válida seguinte para o mesmo item.
		key := x.Acao + x.Material + strconv.Itoa(x.Item)
		if seen[key] {
			continue
		}
		g := req.cite(x.meta)
		conf := x.conf()
		if conf == "alta" && g == nil {
			conf = "media" // decisão do analista, salvo orientação
		}
		i := x.Item
		if x.Acao == "remover" {
			kinds := map[string]string{"arma": "armas", "droga": "drogas", "veiculo": "veiculos"}
			k, ok := kinds[x.Material]
			if !ok || !req.activeItem(k, i) {
				continue
			}
			seen[key] = true
			out = append(out, Suggestion{Kind: KindRemover, Code: sipom.PendEnvioRecusado, Index: &i,
				Target: req.itemLabel(k, i), Value: "remover da ocorrência (não vai ao SIPOM)", Confianca: conf,
				Motivo: x.Motivo, Guidance: g, Applicable: true,
				Apply: map[string]any{"material": k, "motivo": "Sugerido pela IA (recusa do SIPOM): " + strings.TrimSpace(x.Motivo)}})
			continue
		}
		if x.Material != "veiculo" || !req.activeItem("veiculos", i) {
			continue
		}
		target := req.itemLabel("veiculos", i)
		switch x.Acao {
		case "ano":
			fab, mod := x.AnoFabricacao, x.AnoModelo
			maxYear := time.Now().Year() + 1
			if fab < sipom.AnoMinimoVeiculo || fab > maxYear {
				continue
			}
			// Ano que não está no texto só vale com orientação citada.
			if g == nil && !strings.Contains(text, strconv.Itoa(fab)) {
				continue
			}
			apply := map[string]any{"ano_fabricacao": fab}
			value := fmt.Sprintf("ano de fabricação %d", fab)
			if mod >= fab && mod <= fab+1 {
				apply["ano_modelo"] = mod
				value += fmt.Sprintf(" · modelo %d", mod)
			}
			seen[key] = true
			out = append(out, Suggestion{Kind: KindVeiculoAno, Code: sipom.PendEnvioRecusado, Index: &i, Target: target,
				Value: value, Confianca: conf, Motivo: x.Motivo, Guidance: g, Applicable: true, Apply: apply})
		case "outros":
			desc := strings.Join(strings.Fields(x.Descricao), " ")
			if r := []rune(desc); len(r) > 300 {
				desc = string(r[:300])
			}
			if desc == "" {
				desc = opsreport.OutrosDescricao(o.Vehicles[i])
			}
			seen[key] = true
			out = append(out, Suggestion{Kind: KindVeiculoComoOutros, Code: sipom.PendEnvioRecusado, Index: &i, Target: target,
				Value: "enviar como OUTROS: " + desc, Confianca: conf, Motivo: x.Motivo, Guidance: g, Applicable: true,
				Apply: map[string]any{"descricao": desc}})
		}
	}
	return out, nil
}

// activeItem diz se o item existe na ficha e não foi removido.
func (req *Request) activeItem(kind string, i int) bool {
	o := &req.so.Occurrence
	switch kind {
	case "armas":
		return i >= 0 && i < len(o.Weapons) && !o.Weapons[i].Removed()
	case "drogas":
		return i >= 0 && i < len(o.Drugs) && !o.Drugs[i].Removed()
	case "veiculos":
		return i >= 0 && i < len(o.Vehicles) && !o.Vehicles[i].Removed()
	}
	return false
}

func (req *Request) itemLabel(kind string, i int) string {
	o := &req.so.Occurrence
	switch kind {
	case "armas":
		w := o.Weapons[i]
		return fmt.Sprintf("Arma %d: %s", i+1, strings.TrimSpace(w.Kind+" "+w.Brand+" "+w.Model+" "+w.Caliber))
	case "drogas":
		return fmt.Sprintf("Droga %d: %s", i+1, o.Drugs[i].Description)
	}
	v := o.Vehicles[i]
	return fmt.Sprintf("Veículo %d: %s", i+1, strings.TrimSpace(v.Kind+" "+v.Brand+" "+v.Model+" "+v.Color+" "+v.Plate))
}

// ─── Resposta ───

type answer struct {
	Remover []struct {
		Material string `json:"material"`
		Item     int    `json:"item"`
		meta
	} `json:"remover"`
	Natureza *struct {
		ID *int `json:"natureza_id"`
		meta
	} `json:"natureza"`
	Bairro *struct {
		ID *int `json:"bairro_id"`
		meta
	} `json:"bairro"`
	Pessoas []struct {
		Ref  string `json:"ref"`
		Sexo string `json:"sexo"`
		meta
	} `json:"pessoas"`
	Armas []struct {
		Item      int  `json:"item"`
		TipoID    *int `json:"tipo_id"`
		MarcaID   *int `json:"marca_id"`
		CalibreID *int `json:"calibre_id"`
		meta
	} `json:"armas"`
	Drogas []struct {
		Item       int      `json:"item"`
		DrogaID    *int     `json:"droga_id"`
		Quantidade *float64 `json:"quantidade"`
		meta
	} `json:"drogas"`
	Veiculos []struct {
		Item        int     `json:"item"`
		Situacao    string  `json:"situacao"`
		TipoCodigo  *int    `json:"tipo_codigo"`
		CorCodigo   *int    `json:"cor_codigo"`
		MarcaModelo *int    `json:"marca_modelo_codigo"`
		Placa       *string `json:"placa"`
		OutrosDesc  string  `json:"outros_descricao"`
		meta
	} `json:"veiculos"`
}

type addAnswer struct {
	Adicionar []struct {
		Material  string   `json:"material"`
		Tipo      string   `json:"tipo"`
		Marca     string   `json:"marca"`
		Modelo    string   `json:"modelo"`
		Calibre   string   `json:"calibre"`
		Numero    string   `json:"numero"`
		Descricao string   `json:"descricao"`
		Gramas    *float64 `json:"gramas"`
		Porcoes   int      `json:"porcoes"`
		Placa     string   `json:"placa"`
		Cor       string   `json:"cor"`
		meta
	} `json:"adicionar"`
}

type meta struct {
	Confianca string `json:"confianca"`
	Motivo    string `json:"motivo"`
	// Orientacao: a referência ("G1") da orientação do analista que a IA
	// seguiu; vazio = nenhuma. Texto simples, não nullable: o structured
	// outputs aceita no máximo 16 campos com união de tipos no schema.
	Orientacao string `json:"orientacao"`
}

func (m meta) conf() string {
	switch m.Confianca {
	case "alta", "media", "baixa":
		return m.Confianca
	}
	return "baixa"
}

// Parse confere a resposta contra o catálogo e o que foi pedido e devolve
// as sugestões. Id fora do catálogo vira sugestão não aplicável — nunca
// chega a um endpoint.
func (req *Request) Parse(raw []byte) ([]Suggestion, error) {
	var a answer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	cat, o := req.cat, &req.so.Occurrence
	out := []Suggestion{}

	if req.natCode != "" && a.Natureza != nil {
		s := Suggestion{Kind: KindNatureza, Code: req.natCode, Target: "Natureza SIPOM",
			Confianca: a.Natureza.conf(), Motivo: a.Natureza.Motivo, Guidance: req.cite(a.Natureza.meta), Value: "nenhuma natureza do catálogo serve"}
		if cur := req.so.Sipom.NaturezaID; req.natCode == sipom.PendNatConfirm && cur != nil {
			if n, ok := cat.Natureza(*cur); ok {
				s.Current = n.Nome
			}
		}
		if id := a.Natureza.ID; id != nil {
			if n, ok := cat.Natureza(*id); ok {
				s.Value, s.Applicable = n.Nome, true
				s.Apply = map[string]any{"natureza_id": n.ID}
				s.Agrees = req.natCode == sipom.PendNatConfirm && req.so.Sipom.NaturezaID != nil && *req.so.Sipom.NaturezaID == n.ID
			} else {
				s.Value = fmt.Sprintf("id %d inexistente no catálogo", *id)
			}
		}
		out = append(out, s)
	}

	if req.cidadeID != 0 && a.Bairro != nil {
		s := Suggestion{Kind: KindBairro, Code: sipom.PendBairroCatalogo,
			Target:  "Bairro (" + o.PlaceNeighborhood + ")",
			Current: o.PlaceNeighborhood, Confianca: a.Bairro.conf(), Motivo: a.Bairro.Motivo, Guidance: req.cite(a.Bairro.meta),
			Value: "nenhum bairro do catálogo serve"}
		if id := a.Bairro.ID; id != nil {
			if cat.BairroDaCidade(*id, req.cidadeID) {
				s.Value, s.Applicable = cat.BairroNome(*id), true
				s.Apply = map[string]any{"bairro_id": *id}
			} else {
				s.Value = fmt.Sprintf("id %d não é bairro de %s", *id, cat.CidadeNome(req.cidadeID))
			}
		}
		out = append(out, s)
	}

	done := map[string]bool{}
	for _, p := range a.Pessoas {
		pr, ok := req.people[p.Ref]
		if !ok || done[p.Ref] {
			continue
		}
		done[p.Ref] = true
		s := Suggestion{Kind: KindSexo, Code: pr.code, Target: "Envolvido: " + pr.name, PersonID: pr.id,
			Confianca: p.conf(), Motivo: p.Motivo, Guidance: req.cite(p.meta), Value: "indefinido"}
		if pr.suggestion != 0 {
			s.Current = sexoNome(pr.suggestion)
		}
		sexo := map[string]int{"M": sipom.SexoMasculino, "F": sipom.SexoFeminino}[p.Sexo]
		if sexo != 0 {
			s.Value, s.Applicable = sexoNome(sexo), true
			s.Apply = map[string]any{"sexo": sexo}
			s.Agrees = pr.suggestion == sexo
		}
		out = append(out, s)
	}

	asked := func(list []int, i int) bool {
		for _, x := range list {
			if x == i {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	once := func(kind string, i int) bool {
		k := kind + strconv.Itoa(i)
		if seen[k] {
			return false
		}
		seen[k] = true
		return true
	}

	for _, w := range a.Armas {
		if !asked(req.weapons, w.Item) || !once(KindArma, w.Item) {
			continue
		}
		wp := o.Weapons[w.Item]
		i := w.Item
		s := Suggestion{Kind: KindArma, Code: sipom.PendArma, Index: &i,
			Target:    fmt.Sprintf("Arma %d: %s", i+1, strings.TrimSpace(wp.Kind+" "+wp.Brand+" "+wp.Model+" "+wp.Caliber)),
			Confianca: w.conf(), Motivo: w.Motivo, Guidance: req.cite(w.meta), Value: "tipo não identificado"}
		if w.TipoID != nil && cat.ArmaTipoNome(*w.TipoID) != "" {
			parts := []string{cat.ArmaTipoNome(*w.TipoID)}
			apply := map[string]any{"tipo_id": *w.TipoID}
			if w.MarcaID != nil && cat.ArmaMarcaNome(*w.MarcaID) != "" {
				parts = append(parts, cat.ArmaMarcaNome(*w.MarcaID))
				apply["marca_id"] = *w.MarcaID
			}
			if w.CalibreID != nil && cat.ArmaCalibreNome(*w.CalibreID) != "" {
				parts = append(parts, cat.ArmaCalibreNome(*w.CalibreID))
				apply["calibre_id"] = *w.CalibreID
			}
			s.Value, s.Applicable, s.Apply = strings.Join(parts, " · "), true, apply
		}
		out = append(out, s)
	}

	for _, d := range a.Drogas {
		if !asked(req.drugs, d.Item) || !once(KindDroga, d.Item) {
			continue
		}
		i := d.Item
		s := Suggestion{Kind: KindDroga, Code: sipom.PendDroga, Index: &i,
			Target:    fmt.Sprintf("Droga %d: %s", i+1, o.Drugs[i].Description),
			Confianca: d.conf(), Motivo: d.Motivo, Guidance: req.cite(d.meta), Value: "droga não identificada"}
		if d.DrogaID != nil {
			if dr, ok := cat.Droga(*d.DrogaID); ok {
				s.Value = dr.Nome + " · quantidade não informada"
				if d.Quantidade != nil && *d.Quantidade > 0 {
					q := math.Max(1, math.Round(*d.Quantidade))
					s.Value = fmt.Sprintf("%s · %g %s", dr.Nome, q, dr.Unidade)
					s.Applicable = true
					s.Apply = map[string]any{"droga_id": dr.ID, "quantidade": q}
				}
			}
		}
		out = append(out, s)
	}

	for _, v := range a.Veiculos {
		ask, ok := req.vehicles[v.Item]
		if !ok || !once(KindVeiculo, v.Item) {
			continue
		}
		i := v.Item
		vh := o.Vehicles[i]
		target := fmt.Sprintf("Veículo %d: %s", i+1, strings.TrimSpace(vh.Kind+" "+vh.Brand+" "+vh.Model+" "+vh.Color))
		foundPlate := ""
		if ask.plate && v.Placa != nil {
			if p := normalizePlate(*v.Placa); p != "" && strings.Contains(plateKey(o.Narrative+" "+o.SeizedObjects), p) {
				foundPlate = p
			}
		}
		// Sem placa, tipo/cor/marca só importam se a placa apareceu no texto.
		if ask.material && (!ask.plate || foundPlate != "") {
			s := Suggestion{Kind: KindVeiculo, Code: sipom.PendVeiculo, Index: &i, Target: target,
				Confianca: v.conf(), Motivo: v.Motivo, Guidance: req.cite(v.meta)}
			situ := map[string]int{"apreendido": sipom.VeiculoApreendido, "recuperado": sipom.VeiculoRecuperado}[v.Situacao]
			var parts, missing []string
			if situ != 0 {
				parts = append(parts, v.Situacao)
			} else {
				missing = append(missing, "situação")
			}
			tipo := validCode(v.TipoCodigo, cat.VeiculoTipoNome)
			if tipo != "" {
				parts = append(parts, tipo)
			} else {
				missing = append(missing, "tipo")
			}
			cor := validCode(v.CorCodigo, cat.VeiculoCorNome)
			if cor != "" {
				parts = append(parts, cor)
			} else {
				missing = append(missing, "cor")
			}
			var mm string
			if v.MarcaModelo != nil && ask.candidates[*v.MarcaModelo] {
				if m, ok := cat.MarcaModelo(*v.MarcaModelo); ok {
					mm = m.Descricao
				}
			}
			if mm != "" {
				parts = append(parts, mm)
				// Versão (cilindrada, ano, motor) que o texto não traz é
				// palpite entre linhas parecidas: no máximo confiança média.
				if s.Confianca == "alta" && !versionInText(mm, vh.Brand+" "+vh.Model+" "+o.Narrative+" "+o.SeizedObjects) {
					s.Confianca = "media"
				}
			} else {
				missing = append(missing, "marca/modelo")
			}
			s.Value = strings.Join(parts, " · ")
			if len(missing) == 0 {
				s.Applicable = true
				s.Apply = map[string]any{"situacao": situ, "tipo_codigo": *v.TipoCodigo,
					"cor_codigo": *v.CorCodigo, "marca_modelo_codigo": *v.MarcaModelo}
			} else {
				if s.Value != "" {
					s.Value += " · "
				}
				s.Value += "falta " + strings.Join(missing, ", ")
			}
			out = append(out, s)
		}
		switch {
		case foundPlate != "":
			out = append(out, Suggestion{Kind: KindVeiculoPlaca, Code: sipom.PendVeiculoPlaca, Index: &i,
				Target: target, Value: "placa " + foundPlate, Confianca: v.conf(), Motivo: v.Motivo, Guidance: req.cite(v.meta),
				Applicable: true, Apply: map[string]any{"placa": foundPlate}})
		case ask.plate:
			desc := strings.Join(strings.Fields(v.OutrosDesc), " ")
			if r := []rune(desc); len(r) > 300 {
				desc = string(r[:300])
			}
			if desc == "" {
				desc = opsreport.OutrosDescricao(vh)
			}
			// Dizer que a placa não pôde ser identificada é do analista: a
			// sugestão nunca vem marcada (no máximo confiança média).
			conf := v.conf()
			if conf == "alta" {
				conf = "media"
			}
			out = append(out, Suggestion{Kind: KindVeiculoOutros, Code: sipom.PendVeiculoPlaca, Index: &i,
				Target: target, Value: "sem placa → OUTROS: " + desc, Confianca: conf,
				Motivo: "A placa não aparece no relatório. " + v.Motivo, Guidance: req.cite(v.meta),
				Applicable: true, Apply: map[string]any{"descricao": desc}})
		}
	}
	for _, x := range a.Remover {
		if !once(KindRemover+x.Material, x.Item) {
			continue
		}
		var kind, target string
		var removed bool
		i := x.Item
		switch x.Material {
		case "arma":
			if i < 0 || i >= len(o.Weapons) {
				continue
			}
			w := o.Weapons[i]
			kind, removed = "armas", w.Removed()
			target = fmt.Sprintf("Arma %d: %s", i+1, strings.TrimSpace(w.Kind+" "+w.Brand+" "+w.Model+" "+w.Caliber))
		case "droga":
			if i < 0 || i >= len(o.Drugs) {
				continue
			}
			kind, removed = "drogas", o.Drugs[i].Removed()
			target = fmt.Sprintf("Droga %d: %s", i+1, o.Drugs[i].Description)
		case "veiculo":
			if i < 0 || i >= len(o.Vehicles) {
				continue
			}
			v := o.Vehicles[i]
			kind, removed = "veiculos", v.Removed()
			target = fmt.Sprintf("Veículo %d: %s", i+1, strings.TrimSpace(v.Kind+" "+v.Brand+" "+v.Model+" "+v.Color+" "+v.Plate))
		default:
			continue
		}
		if removed {
			continue
		}
		g := req.cite(x.meta)
		// Remover é decisão do analista: só vem marcada com confiança alta
		// quando segue uma orientação.
		conf := x.conf()
		if conf == "alta" && g == nil {
			conf = "media"
		}
		motivo := strings.TrimSpace(x.Motivo)
		if r := []rune(motivo); len(r) > 400 {
			motivo = string(r[:400])
		}
		out = append(out, Suggestion{Kind: KindRemover, Code: "remocao", Index: &i, Target: target,
			Value: "remover da ocorrência (não vai ao SIPOM)", Confianca: conf, Motivo: motivo, Guidance: g,
			Applicable: true, Apply: map[string]any{"material": kind, "motivo": "Sugerido pela IA: " + motivo}})
	}
	return out, nil
}

// ParseAdd confere a resposta da segunda chamada (apreensões que faltam).
func (req *Request) ParseAdd(raw []byte) ([]Suggestion, error) {
	var a addAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	o := &req.so.Occurrence
	out := []Suggestion{}
	for _, x := range a.Adicionar {
		f := func(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), " ")) }
		apply := map[string]any{}
		var target, value string
		switch x.Material {
		case "arma":
			if f(x.Tipo) == "" && f(x.Modelo) == "" {
				continue
			}
			target, apply["material"] = "Nova arma", "armas"
			apply["tipo"], apply["marca"], apply["modelo"], apply["calibre"], apply["numero"] = f(x.Tipo), f(x.Marca), f(x.Modelo), f(x.Calibre), f(x.Numero)
			value = strings.Join(strings.Fields(f(x.Tipo)+" "+f(x.Marca)+" "+f(x.Modelo)+" "+f(x.Calibre)), " ")
			if n := f(x.Numero); n != "" {
				value += " nº " + n
			}
		case "droga":
			d := f(x.Descricao)
			if d == "" || req.hasDrug(d) {
				continue
			}
			target, apply["material"], apply["descricao"] = "Nova droga", "drogas", d
			value = d
			if x.Gramas != nil && *x.Gramas > 0 {
				apply["gramas"] = *x.Gramas
				value += fmt.Sprintf(" · %g g", *x.Gramas)
			}
			if x.Porcoes > 0 {
				apply["porcoes"] = x.Porcoes
				value += fmt.Sprintf(" · %d porção(ões)", x.Porcoes)
			}
		case "veiculo":
			if f(x.Tipo) == "" && f(x.Modelo) == "" {
				continue
			}
			placa := normalizePlate(x.Placa)
			if placa != "" && !strings.Contains(plateKey(o.Narrative+" "+o.SeizedObjects), placa) {
				placa = "" // placa que não está no texto não entra
			}
			if placa != "" && req.hasPlate(placa) {
				continue
			}
			target, apply["material"] = "Novo veículo", "veiculos"
			apply["tipo"], apply["marca"], apply["modelo"], apply["placa"], apply["cor"] = f(x.Tipo), f(x.Marca), f(x.Modelo), placa, f(x.Cor)
			value = strings.Join(strings.Fields(f(x.Tipo)+" "+f(x.Marca)+" "+f(x.Modelo)+" "+f(x.Cor)), " ")
			if placa != "" {
				value += " · placa " + placa
			}
		default:
			continue
		}
		g := req.cite(x.meta)
		conf := x.conf()
		if conf == "alta" && g == nil {
			conf = "media" // incluir é decisão do analista, salvo orientação
		}
		out = append(out, Suggestion{Kind: KindAdicionar, Code: "inclusao", Target: target,
			Value: "incluir: " + value, Confianca: conf, Motivo: x.Motivo, Guidance: g,
			Applicable: true, Apply: apply})
	}
	return out, nil
}

// hasDrug diz se a ficha já tem (ativa) uma droga com a descrição.
func (req *Request) hasDrug(desc string) bool {
	for _, d := range req.so.Drugs {
		if !d.Removed() && strings.EqualFold(strings.TrimSpace(d.Description), desc) {
			return true
		}
	}
	return false
}

// hasPlate diz se a ficha já tem (ativo) um veículo com a placa.
func (req *Request) hasPlate(p string) bool {
	for _, v := range req.so.Vehicles {
		if !v.Removed() && plateKey(v.Plate) == p {
			return true
		}
	}
	return false
}

// versionInText diz se os trechos com algarismo da descrição DENATRAN
// ("125", "1.0", "16V") aparecem no texto do relatório.
func versionInText(desc, text string) bool {
	have := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToUpper(text), notAlnum) {
		have[w] = true
	}
	for _, w := range strings.FieldsFunc(strings.ToUpper(desc), notAlnum) {
		if strings.ContainsAny(w, "0123456789") && !have[w] {
			return false
		}
	}
	return true
}

func notAlnum(r rune) bool {
	return !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9')
}

func validCode(code *int, name func(int) string) string {
	if code == nil {
		return ""
	}
	return name(*code)
}

func sexoNome(s int) string {
	switch s {
	case sipom.SexoMasculino:
		return "Masculino"
	case sipom.SexoFeminino:
		return "Feminino"
	}
	return ""
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

var rePlate = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`)

// plateKey deixa só letras e algarismos, em maiúsculas.
func plateKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizePlate aceita o padrão antigo (ABC1234) e o Mercosul (ABC1D23).
func normalizePlate(s string) string {
	p := plateKey(s)
	if rePlate.MatchString(p) {
		return p
	}
	return ""
}

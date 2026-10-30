package opsreport

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/belia/tevunah/backend/internal/sipom"
)

// NotReadyError: a ocorrência tem pendência que impede o envio.
type NotReadyError struct {
	Pending []string
}

func (e *NotReadyError) Error() string {
	return "ocorrência com pendências para o SIPOM: " + strings.Join(e.Pending, ", ")
}

// PayloadSources são as fontes das fotos do envio. Com Preview, as fotos
// entram sem o conteúdo (base64 vazio) — é a prévia da tela.
type PayloadSources struct {
	// Photo devolve a foto principal do dossiê (nil se não houver ou não
	// puder ir) e o tipo dela.
	Photo func(entityID string) (data []byte, mime string)
	// OccPhoto devolve o arquivo de uma foto da ocorrência (nil se sumiu).
	OccPhoto func(p Photo) []byte
	// Shrink regrava a imagem em JPEG menor (nil = não sabe reduzir).
	Shrink func(data []byte) ([]byte, error)
	// Preview: não lê o conteúdo das fotos.
	Preview bool
}

// Fuso do Ceará: -03:00 o ano todo (sem horário de verão desde 2003).
const ceOffset = "-03:00"

// photoBudget é quanto as fotos podem somar (antes do base64, que cresce
// 4/3) para o corpo caber nos 20 MB do SIPOM com folga para o resto.
const photoBudget = (sipom.MaxBodyBytes - 2<<20) / 4 * 3

// BuildSipomPayload monta o corpo do POST ao SIPOM a partir da ocorrência
// gravada e da tradução já resolvida. Só monta ocorrência pronta: com
// pendência bloqueante devolve *NotReadyError.
func BuildSipomPayload(cat *sipom.Catalog, so *StoredOccurrence, src PayloadSources) (*sipom.Payload, error) {
	f := so.Sipom
	var blocking []string
	for _, p := range f.Pending {
		// A recusa anterior não impede a nova tentativa pela ficha.
		if sipom.Blocking(p) && p != sipom.PendEnvioRecusado {
			blocking = append(blocking, p)
		}
	}
	// O que o SIPOM exige e a tradução pode não ter marcado (geocodificador
	// desligado não deixa pendência de coordenada).
	if !so.Geo.Located() && !hasString(blocking, sipom.PendCoordenada) {
		blocking = append(blocking, sipom.PendCoordenada)
	}
	if f.CidadeID != nil && f.BairroID == nil && !hasString(blocking, sipom.PendBairro) &&
		!hasString(blocking, sipom.PendBairroCatalogo) {
		blocking = append(blocking, sipom.PendBairroCatalogo)
	}
	if len(blocking) == 0 && (f.NaturezaID == nil || f.AreaID == nil || f.OPMID == nil || f.CidadeID == nil || f.BairroID == nil) {
		blocking = []string{"traducao_incompleta"}
	}
	if len(blocking) > 0 {
		return nil, &NotReadyError{Pending: blocking}
	}

	// Número gravado antes da regra (só numeral ou S/N) sai vazio.
	numeral, _ := sipom.NormalizeNumeral(f.Numeral)
	p := &sipom.Payload{
		Origem: sipom.PayloadOrigem{ID: so.ID},
		Ocorrencia: sipom.PayloadOcorrencia{
			DataHora:         fmt.Sprintf("%sT%s:00%s", so.OccurredOn.Format("2006-01-02"), so.StartTime, ceOffset),
			NaturezaFatoID:   *f.NaturezaID,
			UnidadeMilitarID: *f.AreaID,
			OPMMilitarID:     *f.OPMID,
			Endereco: sipom.PayloadEndereco{
				CidadeID: *f.CidadeID, BairroID: *f.BairroID,
				Latitude: *so.Geo.Lat, Longitude: *so.Geo.Lng,
				Logradouro: f.Logradouro, Numeral: numeral,
			},
			NumeroOcorrencia: strings.TrimSpace(so.CIOPS),
		},
		Historico: strings.TrimSpace(so.Narrative),
		// Aba Inteligência: só o que o analista escreveu, e só com a
		// participação da inteligência marcada (vazio não vai).
		HistoricoInteligencia: intelHistory(so),
		Composicao:            []sipom.PayloadComposicao{},
		Procedimentos:         []sipom.PayloadProcedimento{},
		Materiais:             []sipom.PayloadMaterial{},
		Fotos:                 []sipom.PayloadFoto{},
		Envolvidos:            []sipom.PayloadEnvolvido{},
	}

	seen := map[string]bool{}
	for _, o := range so.Officers {
		if o.SipomPoliciamentoTipoID == nil || o.SipomFuncaoID == nil || strings.TrimSpace(o.Registration) == "" {
			continue // composição pendente: vai sem a guarnição
		}
		seen[RegistrationKey(o.Registration)] = true
		p.Composicao = append(p.Composicao, sipom.PayloadComposicao{
			ID: o.ID, Matricula: strings.TrimSpace(o.Registration),
			PoliciamentoTipoID: *o.SipomPoliciamentoTipoID, FuncaoID: *o.SipomFuncaoID,
		})
	}
	// Participação da inteligência: os agentes da agência entram junto com a
	// guarnição, no policiamento de Inteligência.
	p.Composicao = append(p.Composicao, intelPayload(so, seen)...)

	// Procedimentos: delegacia e delegado são os mesmos para todos; delegado
	// é opcional. O id estável é o próprio procedimento (tipo, número, ano).
	if f.ProcedimentoID != nil && f.DelegaciaID != nil {
		res := cat.ResolveProcedure(sipom.ProcInput{Number: so.ProcedureNumber, Type: so.ProcedureType,
			ProcedimentoID: f.ProcedimentoID, ManualProc: true, DelegaciaID: f.DelegaciaID, ManualDelegacia: true,
			ManualDelegado: true})
		for _, it := range res.Itens {
			if it.ProcedimentoID == nil || it.Numero == "" {
				continue
			}
			pp := sipom.PayloadProcedimento{
				ID:             fmt.Sprintf("proc-%d-%s-%s", *it.ProcedimentoID, it.Numero, it.Ano),
				ProcedimentoID: *it.ProcedimentoID, Numero: it.Numero, Ano: it.Ano,
				Reparticao: sipom.ReparticaoPoliciaCivil, DelegaciaID: *f.DelegaciaID,
			}
			if f.DelegadoID != nil {
				if d, ok := cat.Delegado(*f.DelegadoID); ok {
					id := d.ID
					pp.DelegadoID = &id
				}
			}
			p.Procedimentos = append(p.Procedimentos, pp)
		}
	}

	// Materiais, um item por arma, droga e veículo. Item sem tradução
	// completa não chega aqui: é pendência bloqueante.
	for _, w := range so.Weapons {
		if w.SipomTipoID == nil {
			continue
		}
		p.Materiais = append(p.Materiais, sipom.PayloadMaterial{
			ID: w.ID, TipoID: sipom.MaterialArma,
			ArmaTipoID: w.SipomTipoID, ArmaMarcaID: w.SipomMarcaID, ArmaCalibreID: w.SipomCalibreID,
			Numero: strings.TrimSpace(w.Serial), Descricao: strings.TrimSpace(w.Model),
		})
	}
	for _, d := range so.Drugs {
		if d.SipomDrogaID == nil || d.SipomQuantidade == nil {
			continue
		}
		id, q := *d.SipomDrogaID, *d.SipomQuantidade
		p.Materiais = append(p.Materiais, sipom.PayloadMaterial{
			ID: d.ID, TipoID: sipom.MaterialDroga, DrogaTipoID: &id, Quantidade: &q,
		})
	}
	// Veículo vai com placa e situação e com o que mais estiver resolvido
	// (o SIPOM ignora para placa já cadastrada; para placa nova, recusa se
	// faltar — ver VehiclePending).
	for _, v := range so.Vehicles {
		if VehiclePending(v) != "" {
			continue
		}
		sit := v.SipomSituacao
		p.Materiais = append(p.Materiais, sipom.PayloadMaterial{
			ID: v.ID, TipoID: sipom.MaterialVeiculo, Placa: plate(v.Plate), Situacao: &sit,
			CodigoTipo: v.SipomTipoCodigo, CodigoCor: v.SipomCorCodigo, CodigoMarcaModelo: v.SipomMarcaModeloCodigo,
			AnoFabricacao: v.SipomAnoFabricacao, AnoModelo: v.SipomAnoModelo,
		})
	}

	// Envolvidos. Identificado vai com a pessoa (nome e sexo obrigatórios);
	// não identificado, sem ela.
	var pend []string
	var photos []payloadPhoto
	for _, sp := range so.People {
		tp := sipom.TranslatePerson(personInput(so, sp))
		vid, ok := cat.VinculoID(tp.Vinculo)
		if !ok {
			pend = appendOnce(pend, sipom.PendCatalogoVinculo)
			continue
		}
		e := sipom.PayloadEnvolvido{ID: sp.ID, VinculoID: vid, Morte: tp.Morte}
		if tp.Nome == "" {
			e.Sexo = tp.Sexo
			p.Envolvidos = append(p.Envolvidos, e)
			continue
		}
		if tp.Sexo == 0 {
			if tp.SexoSugerido != 0 {
				pend = appendOnce(pend, sipom.PendPessoaSexoConfirm)
			} else {
				pend = appendOnce(pend, sipom.PendPessoaSexo)
			}
			continue
		}
		nome := tp.Nome
		if utf8.RuneCountInString(nome) > 125 {
			nome = string([]rune(nome)[:125])
		}
		pe := &sipom.PayloadPessoa{Nome: nome, Sexo: tp.Sexo, Mae: tp.Mae}
		if len(tp.CPF) == 11 {
			pe.CPF = tp.CPF
		}
		if tp.Nascimento != nil {
			pe.Nascimento = tp.Nascimento.Format("2006-01-02")
		}
		if tp.Foto && sp.EntityID != nil && src.Photo != nil {
			if data, mime := src.Photo(*sp.EntityID); data != nil || (src.Preview && mime != "") {
				photos = append(photos, payloadPhoto{data: data, mime: mime, set: func(b64, mime string) {
					pe.Foto = &sipom.PayloadFotoMime{Mime: mime, Base64: b64}
				}})
			}
		}
		e.Pessoa = pe
		p.Envolvidos = append(p.Envolvidos, e)
	}
	if len(pend) > 0 {
		return nil, &NotReadyError{Pending: pend}
	}

	if src.OccPhoto != nil {
		for _, ph := range so.Photos {
			if len(p.Fotos) >= sipom.MaxFotos {
				break
			}
			data := src.OccPhoto(ph)
			if data == nil && !src.Preview {
				continue
			}
			i := len(p.Fotos)
			p.Fotos = append(p.Fotos, sipom.PayloadFoto{ID: ph.ID})
			photos = append(photos, payloadPhoto{data: data, mime: ph.MIME, set: func(b64, _ string) {
				p.Fotos[i].Base64 = b64
			}})
		}
	}
	if err := fitPhotos(photos, src.Shrink); err != nil {
		return nil, err
	}

	if len(p.Composicao) > sipom.MaxComposicao || len(p.Procedimentos) > sipom.MaxProcedimentos ||
		len(p.Materiais) > sipom.MaxMateriais || len(p.Envolvidos) > sipom.MaxEnvolvidos {
		return nil, fmt.Errorf("ocorrência acima dos limites do SIPOM (composição %d/30, procedimentos %d/20, materiais %d/100, envolvidos %d/50)",
			len(p.Composicao), len(p.Procedimentos), len(p.Materiais), len(p.Envolvidos))
	}
	return p, nil
}

// payloadPhoto é uma foto a caminho do payload: o conteúdo e onde gravá-lo
// (foto da pessoa ou da ocorrência).
type payloadPhoto struct {
	data []byte
	mime string
	set  func(b64, mime string)
}

// fitPhotos põe as fotos no payload dentro dos limites do SIPOM: cada uma
// até 5 MB, e todas juntas dentro de photoBudget. Passou, regrava em JPEG
// menor (Shrink); sem Shrink, ou se nem assim couber, é erro — melhor que
// mandar e o SIPOM recusar com 413.
func fitPhotos(list []payloadPhoto, shrink func([]byte) ([]byte, error)) error {
	total := 0
	for _, x := range list {
		total += len(x.data)
	}
	for i := range list {
		if len(list[i].data) <= sipom.MaxFotoBytes && total <= photoBudget {
			continue
		}
		if shrink == nil {
			return fmt.Errorf("fotos acima do limite do SIPOM (5 MB cada, 20 MB o envio)")
		}
		small, err := shrink(list[i].data)
		if err != nil {
			return fmt.Errorf("reduzir foto para o SIPOM: %w", err)
		}
		total += len(small) - len(list[i].data)
		list[i].data, list[i].mime = small, "image/jpeg"
	}
	if total > photoBudget {
		return fmt.Errorf("fotos somam %d MB mesmo reduzidas — acima do limite do envio ao SIPOM", total>>20)
	}
	for _, x := range list {
		b64 := ""
		if x.data != nil {
			b64 = base64.StdEncoding.EncodeToString(x.data)
		}
		x.set(b64, x.mime)
	}
	return nil
}

// plate normaliza a placa como o SIPOM a cadastra: letras e algarismos, em
// maiúsculas ("pmx-2a41" → "PMX2A41").
func plate(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func appendOnce(ss []string, s string) []string {
	if hasString(ss, s) {
		return ss
	}
	return append(ss, s)
}

// personInput junta o envolvido do relatório com o dossiê vinculado.
func personInput(so *StoredOccurrence, sp StoredPerson) sipom.PersonInput {
	return sipom.PersonInput{
		Role: sp.Role, Name: sp.Name, MotherName: sp.MotherName, Note: sp.Note,
		Linked: sp.EntityID != nil, EntName: sp.EntityName, EntMother: sp.EntityMother,
		CPF: sp.EntityCPF, Gender: sp.EntityGender, BirthDate: sp.EntityBirthDate,
		Deceased: sp.EntityDeceased, DeceasedOn: sp.EntityDeceasedOn, HasPhoto: sp.EntityHasPhoto,
		Sexo: sp.SipomSexo, SexoSugerido: sp.SipomSexoSugerido, SexoFonte: sp.SipomSexoFonte,
		OccurredOn: so.OccurredOn,
	}
}

// intelHistory é o histórico de inteligência do envio. Desmarcada a
// participação, o texto fica guardado na ficha mas não vai ao SIPOM.
func intelHistory(so *StoredOccurrence) string {
	if !so.IntelParticipation {
		return ""
	}
	return strings.TrimSpace(so.IntelHistory)
}

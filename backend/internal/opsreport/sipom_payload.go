package opsreport

import (
	"fmt"
	"strings"

	"github.com/belia/tevunah/backend/internal/sipom"
)

// NotReadyError: a ocorrência tem pendência que impede o envio.
type NotReadyError struct {
	Pending []string
}

func (e *NotReadyError) Error() string {
	return "ocorrência com pendências para o SIPOM: " + strings.Join(e.Pending, ", ")
}

// PhotoLoader devolve a foto principal do dossiê (nil se não houver ou não
// couber no envio).
type PhotoLoader func(entityID string) *sipom.PayloadFoto

// Fuso do Ceará: -03:00 o ano todo (sem horário de verão desde 2003).
const ceOffset = "-03:00"

// BuildSipomPayload monta o corpo do POST ao SIPOM a partir da ocorrência
// gravada e da tradução já resolvida. Só monta ocorrência pronta: com
// pendência bloqueante devolve *NotReadyError.
func BuildSipomPayload(cat *sipom.Catalog, so *StoredOccurrence, unidade string, photo PhotoLoader) (*sipom.Payload, error) {
	f := so.Sipom
	var blocking []string
	for _, p := range f.Pending {
		if sipom.Blocking(p) {
			blocking = append(blocking, p)
		}
	}
	if len(blocking) > 0 || f.NaturezaID == nil || f.AreaID == nil || f.OPMID == nil || f.CidadeID == nil {
		if len(blocking) == 0 {
			blocking = []string{"traducao_incompleta"}
		}
		return nil, &NotReadyError{Pending: blocking}
	}

	nat, _ := cat.Natureza(*f.NaturezaID)
	area, _ := cat.Companhia(*f.AreaID)
	opm, _ := cat.Companhia(*f.OPMID)
	bairro := so.PlaceNeighborhood
	if f.BairroID != nil {
		bairro = cat.BairroNome(*f.BairroID)
	}
	p := &sipom.Payload{
		Origem: sipom.PayloadOrigem{
			Sistema: "TEVUNAH", Versao: sipom.PayloadVersion, Unidade: unidade, ID: so.ID,
		},
		Ocorrencia: sipom.PayloadOcorrencia{
			NaturezaFatoID:   nat.ID,
			NaturezaFato:     nat.Nome,
			DataHora:         fmt.Sprintf("%sT%s:00%s", so.OccurredOn.Format("2006-01-02"), so.StartTime, ceOffset),
			UnidadeMilitarID: area.ID,
			UnidadeMilitar:   area.Abreviado,
			OPMMilitarID:     opm.ID,
			OPMMilitar:       opm.Abreviado,
			Endereco: sipom.PayloadEndereco{
				Logradouro: f.Logradouro,
				Numeral:    f.Numeral,
				Bairro:     strings.ToUpper(bairro),
				BairroID:   f.BairroID,
				Cidade:     strings.ToUpper(cat.CidadeNome(*f.CidadeID)),
				CidadeID:   *f.CidadeID,
				CidadeIBGE: cat.CidadeIBGE(*f.CidadeID),
				Latitude:   so.Geo.Lat,
				Longitude:  so.Geo.Lng,
			},
			NumeroOcorrencia:         so.CIOPS,
			ParticipacaoInteligencia: so.IntelParticipation,
		},
		Historico:  so.Narrative,
		Envolvidos: []sipom.PayloadEnvolvido{},
		Composicao: []sipom.PayloadComposicao{},
		Materiais:  []sipom.PayloadMaterial{},
	}
	if so.Report != nil && !so.Report.IsZero() {
		d := so.Report.Format("2006-01-02")
		p.Origem.RelatorioData = &d
	}

	for _, sp := range so.People {
		tp := sipom.TranslatePerson(personInput(so, sp))
		e := sipom.PayloadEnvolvido{
			Vinculo: sipom.PayloadVinculo[tp.Vinculo],
			Nome:    strPtr(tp.Nome), CPF: strPtr(tp.CPF), Mae: strPtr(tp.Mae),
			Morte: tp.Morte,
		}
		if tp.Sexo != 0 {
			s := tp.Sexo
			e.Sexo = &s
		}
		if tp.Nascimento != nil {
			d := tp.Nascimento.Format("2006-01-02")
			e.Nascimento = &d
		}
		if tp.Foto && sp.EntityID != nil && photo != nil {
			e.Foto = photo(*sp.EntityID)
		}
		p.Envolvidos = append(p.Envolvidos, e)
	}

	// Procedimento: só vai quando o relatório trouxe (sem ele a ocorrência
	// está pendente e não chega aqui); delegado é opcional.
	if f.ProcedimentoID != nil && f.DelegaciaID != nil {
		procName, _ := cat.Procedimento(*f.ProcedimentoID)
		dele, _ := cat.Delegacia(*f.DelegaciaID)
		num := cat.ResolveProcedure(sipom.ProcInput{Number: so.ProcedureNumber, Type: so.ProcedureType,
			ProcedimentoID: f.ProcedimentoID, ManualProc: true, DelegaciaID: f.DelegaciaID, ManualDelegacia: true,
			ManualDelegado: true})
		p.Procedimento = &sipom.PayloadProcedimento{
			Reparticao: sipom.ReparticaoPoliciaCivil, ProcedimentoID: *f.ProcedimentoID, Procedimento: procName,
			Numero: num.Numero, Ano: num.Ano, DelegaciaID: dele.ID, Delegacia: dele.Nome,
		}
		if f.DelegadoID != nil {
			if d, ok := cat.Delegado(*f.DelegadoID); ok {
				id := d.ID
				p.Procedimento.DelegadoID, p.Procedimento.Delegado = &id, &d.Nome
			}
		}
	}

	// Materiais, um item por arma, droga e veículo. Item sem tradução
	// completa não chega aqui: é pendência bloqueante.
	for _, w := range so.Weapons {
		if w.SipomTipoID == nil || w.SipomMarcaID == nil || w.SipomCalibreID == nil {
			continue
		}
		one := 1.0
		p.Materiais = append(p.Materiais, sipom.PayloadMaterial{
			TipoID: sipom.MaterialArma, Tipo: "Arma",
			ArmaTipoID: w.SipomTipoID, ArmaTipo: cat.ArmaTipoNome(*w.SipomTipoID),
			ArmaMarcaID: w.SipomMarcaID, ArmaMarca: cat.ArmaMarcaNome(*w.SipomMarcaID),
			ArmaCalibreID: w.SipomCalibreID, ArmaCalibre: cat.ArmaCalibreNome(*w.SipomCalibreID),
			Numero: w.Serial, Descricao: w.Model, Quantidade: &one,
		})
	}
	for _, d := range so.Drugs {
		if d.SipomDrogaID == nil || d.SipomQuantidade == nil {
			continue
		}
		dr, _ := cat.Droga(*d.SipomDrogaID)
		q := *d.SipomQuantidade
		p.Materiais = append(p.Materiais, sipom.PayloadMaterial{
			TipoID: sipom.MaterialDroga, Tipo: "Droga", Droga: dr.Nome, Unidade: dr.Unidade, Quantidade: &q,
		})
	}
	for _, v := range so.Vehicles {
		if v.SipomTipoCodigo == nil || v.SipomCorCodigo == nil || v.SipomMarcaModeloCodigo == nil || v.SipomSituacao == 0 {
			continue
		}
		mm, _ := cat.MarcaModelo(*v.SipomMarcaModeloCodigo)
		sit := v.SipomSituacao
		p.Materiais = append(p.Materiais, sipom.PayloadMaterial{
			TipoID: sipom.MaterialVeiculo, Tipo: "Veículo", Situacao: &sit, Placa: v.Plate,
			VeiculoTipoCodigo: v.SipomTipoCodigo, VeiculoTipo: cat.VeiculoTipoNome(*v.SipomTipoCodigo),
			MarcaModeloCodigo: v.SipomMarcaModeloCodigo, MarcaModelo: mm.Descricao,
			CorCodigo: v.SipomCorCodigo, Cor: cat.VeiculoCorNome(*v.SipomCorCodigo),
		})
	}

	for _, o := range so.Officers {
		if o.SipomPoliciamentoTipoID == nil || o.SipomFuncaoID == nil {
			continue // composição pendente: vai sem a guarnição
		}
		c := sipom.PayloadComposicao{
			Matricula: o.Registration, PoliciamentoTipoID: *o.SipomPoliciamentoTipoID,
			FuncaoID: *o.SipomFuncaoID, Posto: o.Rank, Numeral: o.Number,
			NomeGuerra: o.WarName, Equipe: o.SipomEquipe,
		}
		if id, ok := cat.Posto(o.Rank); ok {
			c.PostoGraduacaoID = &id
		}
		p.Composicao = append(p.Composicao, c)
	}
	return p, nil
}

// personInput junta o envolvido do relatório com o dossiê vinculado.
func personInput(so *StoredOccurrence, sp StoredPerson) sipom.PersonInput {
	return sipom.PersonInput{
		Role: sp.Role, Name: sp.Name, MotherName: sp.MotherName, Note: sp.Note,
		Linked: sp.EntityID != nil, EntName: sp.EntityName, EntMother: sp.EntityMother,
		CPF: sp.EntityCPF, Gender: sp.EntityGender, BirthDate: sp.EntityBirthDate,
		Deceased: sp.EntityDeceased, DeceasedOn: sp.EntityDeceasedOn, HasPhoto: sp.EntityHasPhoto,
		OccurredOn: so.OccurredOn,
	}
}

func strPtr(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

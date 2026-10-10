package sipom

import (
	"regexp"
	"strings"
)

// Materiais (aba Materiais do SIPOM): armas, drogas e veículos do relatório
// operacional nos códigos das listas do SIPOM. Munição, celulares e dinheiro
// não vêm estruturados do relatório e ficam de fora.

// Situação do veículo no SIPOM.
const (
	VeiculoApreendido = 1
	VeiculoRecuperado = 2
)

// ─── Armas ───

// WeaponInput é a arma como o relatório a descreve, mais o que o analista
// fixou (Manual: nada é recalculado).
type WeaponInput struct {
	Kind, Brand, Model, Caliber string
	TipoID, MarcaID, CalibreID  *int
	Manual                      bool
}

// WeaponResult é a arma nos códigos do SIPOM; OK = os três campos
// obrigatórios resolvidos.
type WeaponResult struct {
	TipoID, MarcaID, CalibreID *int
	OK                         bool
}

// weaponKinds: como o relatório escreve o tipo → nome no catálogo.
var weaponKinds = map[string]string{
	"revolver": "Revolver", "pistola": "Pistola", "fuzil": "Fuzil", "espingarda": "Espingarda",
	"carabina": "Carabina", "rifle": "Rifle", "simulacro": "Simulacro", "simulacro de arma": "Simulacro",
	"arma branca": "Branca", "branca": "Branca", "faca": "Branca", "facao": "Branca", "canivete": "Branca",
	"punhal": "Branca", "machado": "Branca", "foice": "Branca", "peixeira": "Branca",
	"artesanal": "Artesanal", "arma artesanal": "Artesanal", "arma caseira": "Artesanal",
}

func (c *Catalog) ResolveWeapon(in WeaponInput, terms *TermMap) WeaponResult {
	if in.Manual {
		r := WeaponResult{TipoID: in.TipoID, MarcaID: in.MarcaID, CalibreID: in.CalibreID}
		r.OK = r.TipoID != nil && r.MarcaID != nil && r.CalibreID != nil
		return r
	}
	var r WeaponResult
	// Tipo: termo aprendido; senão a tabela de sinônimos; senão "artesanal"
	// em qualquer campo ("SUBMETRALHADORA ARTESANAL").
	if id, ok := terms.Lookup(TermArmaTipo, in.Kind); ok && c.armaTipos.byID[id] != "" {
		r.TipoID = &id
	} else if name, ok := weaponKinds[key(in.Kind)]; ok {
		if id, ok := c.armaTipos.byKey[key(name)]; ok {
			r.TipoID = &id
		}
	} else if id, ok := c.armaTipos.byKey[key(in.Kind)]; ok {
		r.TipoID = &id
	}
	if r.TipoID == nil {
		all := key(in.Kind + " " + in.Model + " " + in.Brand)
		if strings.Contains(all, "artesanal") || strings.Contains(all, "caseir") {
			if id, ok := c.armaTipos.byKey["artesanal"]; ok {
				r.TipoID = &id
			}
		}
	}
	// Marca: termo aprendido; senão o nome do catálogo (inteiro ou a
	// primeira palavra: "BOITO" para "Boito (E.R.Amantino & Cia Ltda)").
	if id, ok := terms.Lookup(TermArmaMarca, in.Brand); ok && c.armaMarcas.byID[id] != "" {
		r.MarcaID = &id
	} else if id, ok := c.matchBrand(in.Brand); ok {
		r.MarcaID = &id
	}
	// Calibre: termo aprendido; senão a forma numérica.
	if id, ok := terms.Lookup(TermArmaCalibre, in.Caliber); ok && c.armaCalibres.byID[id] != "" {
		r.CalibreID = &id
	} else if id, ok := c.matchCaliber(in.Caliber); ok {
		r.CalibreID = &id
	}
	r.OK = r.TipoID != nil && r.MarcaID != nil && r.CalibreID != nil
	return r
}

// matchBrand casa a marca pelo nome inteiro ou pela primeira palavra do
// nome do catálogo, quando ela identifica uma marca só.
func (c *Catalog) matchBrand(brand string) (int, bool) {
	k := key(brand)
	if k == "" {
		return 0, false
	}
	if id, ok := c.armaMarcas.byKey[k]; ok {
		return id, true
	}
	found, n := 0, 0
	for _, m := range c.armaMarcas.items {
		words := strings.Fields(key(m.Nome))
		if len(words) > 0 && words[0] == k {
			found, n = m.ID, n+1
		}
	}
	return found, n == 1
}

var reCaliberDigits = regexp.MustCompile(`[^0-9]`)

// caliberKey reduz o calibre à sequência de dígitos: ".38", "38" e "38 SPL"
// viram "38"; "9mm", "9x19" e "9" viram "9"; "5.56", "5,56" e "556", "556".
func caliberKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ",", ".")
	if i := strings.IndexAny(s, " x/"); i > 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, "mm")
	return reCaliberDigits.ReplaceAllString(s, "")
}

func (c *Catalog) matchCaliber(caliber string) (int, bool) {
	k := caliberKey(caliber)
	if k == "" {
		return 0, false
	}
	for _, it := range c.armaCalibres.items {
		if caliberKey(it.Nome) == k {
			return it.ID, true
		}
	}
	return 0, false
}

// ─── Drogas ───

// DrugInput é a droga como o relatório a descreve.
type DrugInput struct {
	Description string
	Grams       *float64
	DrogaID     *int
	Quantidade  *float64
	Manual      bool
}

// DrugResult é a droga nos códigos do SIPOM. Quantidade é na unidade da
// droga; OK = droga e quantidade resolvidas.
type DrugResult struct {
	DrogaID    *int
	Quantidade *float64
	OK         bool
}

// drugAliases: palavras do relatório → nome da droga no catálogo.
var drugAliases = []struct{ word, name string }{
	{"maconha", "Maconha"}, {"cannabis", "Maconha"}, {"skank", "Skank"}, {"skunk", "Skank"},
	{"cocaina", "Cocaína"}, {"pasta base", "Cocaína"}, {"crack", "Crack"}, {"haxixe", "Haxixe"},
	{"hash", "Haxixe"}, {"ecstasy", "Ecstasy/MDMA"}, {"mdma", "Ecstasy/MDMA"}, {"lsd", "Lsd"},
	{"heroina", "Heroína"}, {"ayahuasca", "Chá de Ayahuasca"}, {"fentanil", "Fentanil"},
	{"quetamina", "Quetamina"}, {"cetamina", "Quetamina"}, {"ketamina", "Quetamina"},
	{"solvente", "Solvente"}, {"lanca perfume", "Solvente"}, {"lolo", "Solvente"},
}

func (c *Catalog) ResolveDrug(in DrugInput, terms *TermMap) DrugResult {
	if in.Manual {
		r := DrugResult{DrogaID: in.DrogaID, Quantidade: in.Quantidade}
		r.OK = r.DrogaID != nil && r.Quantidade != nil
		return r
	}
	var r DrugResult
	if id, ok := terms.Lookup(TermDroga, in.Description); ok {
		if _, exists := c.drogas[id]; exists {
			r.DrogaID = &id
		}
	}
	if r.DrogaID == nil {
		k := " " + key(in.Description) + " "
		for _, a := range drugAliases {
			if strings.Contains(k, " "+a.word+" ") {
				for _, d := range c.drogaList {
					if d.Nome == a.name {
						id := d.ID
						r.DrogaID = &id
					}
				}
				break
			}
		}
	}
	// Quantidade: o relatório traz gramas; só serve para droga medida em
	// gramas. Comprimido, dose ou ml ficam para o analista.
	if r.DrogaID != nil && in.Grams != nil {
		if d := c.drogas[*r.DrogaID]; strings.HasPrefix(key(d.Unidade), "gramas") {
			g := *in.Grams
			r.Quantidade = &g
		}
	}
	r.OK = r.DrogaID != nil && r.Quantidade != nil
	return r
}

// ─── Veículos ───

// VehicleInput é o veículo como o relatório o descreve.
type VehicleInput struct {
	Kind, Brand, Model, Color string
	// Recovered: a ocorrência é de recuperação de veículo (natureza).
	Recovered bool

	TipoCodigo, CorCodigo, MarcaModeloCodigo *int
	Situacao                                 int
	Manual                                   bool
}

// VehicleResult é o veículo nos códigos DENATRAN que o SIPOM usa.
type VehicleResult struct {
	TipoCodigo, CorCodigo, MarcaModeloCodigo *int
	Situacao                                 int
	// MarcaModeloCandidates: linhas que casam parcialmente com marca e
	// modelo quando nenhuma casa inteira — o analista escolhe.
	MarcaModeloCandidates []MarcaModelo
	OK                    bool
}

// vehicleKinds: como o relatório escreve o tipo → código DENATRAN.
var vehicleKinds = map[string]int{
	"moto": 4, "motocicleta": 4, "motociclista": 4, "motocicleta de passeio": 4,
	"motoneta": 3, "ciclomotor": 2, "triciclo": 5, "quadriciclo": 21,
	"automovel": 6, "carro": 6, "veiculo de passeio": 6, "passeio": 6,
	"caminhonete": 23, "pick up": 23, "pickup": 23, "camioneta": 13, "suv": 25, "utilitario": 25,
	"caminhao": 14, "onibus": 8, "micro onibus": 7, "microonibus": 7, "van": 7, "reboque": 10,
}

func (c *Catalog) ResolveVehicle(in VehicleInput, terms *TermMap) VehicleResult {
	if in.Manual {
		r := VehicleResult{TipoCodigo: in.TipoCodigo, CorCodigo: in.CorCodigo,
			MarcaModeloCodigo: in.MarcaModeloCodigo, Situacao: in.Situacao}
		r.OK = r.TipoCodigo != nil && r.CorCodigo != nil && r.MarcaModeloCodigo != nil && r.Situacao != 0
		return r
	}
	var r VehicleResult
	// Tipo.
	if code, ok := terms.Lookup(TermVeiculoTipo, in.Kind); ok && c.veiculoTipos.byCode[code] != "" {
		r.TipoCodigo = &code
	} else if code, ok := vehicleKinds[key(in.Kind)]; ok && c.veiculoTipos.byCode[code] != "" {
		r.TipoCodigo = &code
	} else if code, ok := c.veiculoTipos.byKey[key(in.Kind)]; ok {
		r.TipoCodigo = &code
	}
	// Cor: o catálogo escreve no feminino ("PRETA"); o relatório, como
	// quiser ("PRETO") — compara sem a vogal final.
	if code, ok := terms.Lookup(TermVeiculoCor, in.Color); ok && c.veiculoCores.byCode[code] != "" {
		r.CorCodigo = &code
	} else if code, ok := c.veiculoCores.byKey[key(in.Color)]; ok {
		r.CorCodigo = &code
	} else if stem := colorStem(in.Color); stem != "" {
		for _, it := range c.veiculoCores.items {
			if colorStem(it.Nome) == stem {
				code := it.ID
				r.CorCodigo = &code
				break
			}
		}
	}
	// Marca/modelo: termo aprendido; senão a descrição exata ("HONDA/CG
	// 160 FAN"); senão a busca por palavras — um resultado só é aceito.
	pair := strings.TrimSpace(in.Brand) + "/" + strings.TrimSpace(in.Model)
	if code, ok := terms.Lookup(TermVeiculoMarcaModel, pair); ok {
		if _, exists := c.marcaModeloByCode[code]; exists {
			r.MarcaModeloCodigo = &code
		}
	}
	if r.MarcaModeloCodigo == nil && (in.Brand != "" || in.Model != "") {
		if m, ok := c.marcaModeloKey[key(pair)]; ok {
			code := m.Codigo
			r.MarcaModeloCodigo = &code
		} else {
			cands := c.SearchMarcasModelos(in.Brand+" "+in.Model, 21)
			if len(cands) == 1 {
				code := cands[0].Codigo
				r.MarcaModeloCodigo = &code
			} else if len(cands) > 1 {
				r.MarcaModeloCandidates = cands
			}
		}
	}
	r.Situacao = VeiculoApreendido
	if in.Recovered {
		r.Situacao = VeiculoRecuperado
	}
	r.OK = r.TipoCodigo != nil && r.CorCodigo != nil && r.MarcaModeloCodigo != nil
	return r
}

func colorStem(s string) string {
	k := key(s)
	if len(k) < 4 {
		return ""
	}
	return strings.TrimRight(k, "aeo")
}

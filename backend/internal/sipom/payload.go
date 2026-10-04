package sipom

// Contrato do envio ao SIPOM — o corpo do POST proposto ao desenvolvedor do
// SIPOM (ver docs/sipom-integracao.md). Ids são os das tabelas do SIPOM
// (natureza_fatos, companhias, cidade, bairro, policiamentos_*,
// postos_graduacoes); textos acompanham os ids só para conferência.
//
// Versão do contrato: muda quando um campo muda de sentido ou sai.
const PayloadVersion = "1"

// Payload é uma ocorrência completa: cabeçalho, históricos, envolvidos e
// composição, num POST só (o SIPOM grava tudo ou nada).
type Payload struct {
	Origem     PayloadOrigem     `json:"origem"`
	Ocorrencia PayloadOcorrencia `json:"ocorrencia"`
	Historico  string            `json:"historico"`
	// Reservado: o texto do histórico de inteligência ainda não é gerado
	// pelo Tevunah (vai null).
	HistoricoInteligencia *string             `json:"historico_inteligencia"`
	Envolvidos            []PayloadEnvolvido  `json:"envolvidos"`
	Composicao            []PayloadComposicao `json:"composicao"`
}

// PayloadOrigem identifica o registro no Tevunah. origem.id é a chave de
// idempotência: o mesmo id reenviado não cria outra ocorrência.
type PayloadOrigem struct {
	Sistema       string  `json:"sistema"` // "TEVUNAH"
	Versao        string  `json:"versao"`  // versão do contrato
	Unidade       string  `json:"unidade"` // agência que envia ("SAI/2º BPRAIO")
	ID            string  `json:"id"`      // uuid da ocorrência no Tevunah
	RelatorioData *string `json:"relatorio_data"`
}

// PayloadOcorrencia é o cabeçalho — os campos de "Criando nova ocorrência".
type PayloadOcorrencia struct {
	NaturezaFatoID int    `json:"natureza_fato_id"` // natureza_fatos.id
	NaturezaFato   string `json:"natureza_fato"`    // natureza_fatos.nome (conferência)
	// Data e hora do fato, ISO 8601 com o fuso do Ceará (-03:00).
	DataHora string `json:"data_hora"`
	// Área da unidade militar do local do fato (companhias.id).
	UnidadeMilitarID int    `json:"unidade_militar_id"`
	UnidadeMilitar   string `json:"unidade_militar"`
	// OPM que atendeu (companhias.id).
	OPMMilitarID     int             `json:"opm_militar_id"`
	OPMMilitar       string          `json:"opm_militar"`
	Endereco         PayloadEndereco `json:"endereco"`
	NumeroOcorrencia string          `json:"numero_ocorrencia"` // ficha CIOPS
	Viatura          *string         `json:"viatura"`
	NumeroHT         *string         `json:"numero_ht"`
	// Houve participação da inteligência (SAI) na ocorrência.
	ParticipacaoInteligencia bool `json:"participacao_inteligencia"`
}

// PayloadEndereco: no formulário do SIPOM bairro e cidade são texto; os ids
// do catálogo vão junto para quem quiser casar sem depender da grafia.
type PayloadEndereco struct {
	Logradouro string   `json:"logradouro"`
	Numeral    string   `json:"numeral"`
	Bairro     string   `json:"bairro"`
	BairroID   *int     `json:"bairro_id"` // bairro.id; null se o bairro não está no catálogo
	Cidade     string   `json:"cidade"`
	CidadeID   int      `json:"cidade_id"` // cidade.id
	CidadeIBGE int      `json:"cidade_ibge"`
	Latitude   *float64 `json:"latitude"`
	Longitude  *float64 `json:"longitude"`
}

// Vínculo do envolvido. No formulário do SIPOM o valor da opção é um token
// que muda a cada carregamento, por isso o contrato usa códigos estáveis.
const (
	PayloadVincVitima     = "VITIMA"
	PayloadVincInfrator   = "INFRATOR"
	PayloadVincTestemunha = "TESTEMUNHA"
	PayloadVincVitimaNI   = "VITIMA_NAO_IDENTIFICADA"
	PayloadVincInfratorNI = "INFRATOR_NAO_IDENTIFICADO"
)

// PayloadVinculo traduz o vínculo interno para o código do contrato.
var PayloadVinculo = map[string]string{
	VincVitima:     PayloadVincVitima,
	VincInfrator:   PayloadVincInfrator,
	VincTestemunha: PayloadVincTestemunha,
	VincVitimaNI:   PayloadVincVitimaNI,
	VincInfratorNI: PayloadVincInfratorNI,
}

// PayloadEnvolvido são os campos do modal "Adicionar Pessoa à Ocorrência".
type PayloadEnvolvido struct {
	Vinculo    string       `json:"vinculo"`
	Nome       *string      `json:"nome"`
	CPF        *string      `json:"cpf"`        // 11 dígitos
	Sexo       *int         `json:"sexo"`       // 1 masculino, 2 feminino
	Nascimento *string      `json:"nascimento"` // YYYY-MM-DD
	Mae        *string      `json:"mae"`
	Morte      bool         `json:"morte"`
	Foto       *PayloadFoto `json:"foto"`
}

// PayloadFoto é a foto principal do dossiê, em base64.
type PayloadFoto struct {
	Mime   string `json:"mime"` // image/jpeg | image/png
	Base64 string `json:"base64"`
}

// PayloadComposicao é um policial da guarnição. O SIPOM localiza o servidor
// pela matrícula; posto e nome de guerra vão para conferência.
type PayloadComposicao struct {
	Matricula          string `json:"matricula"`
	PoliciamentoTipoID int    `json:"policiamento_tipo_id"` // policiamentos_tipos.id
	FuncaoID           int    `json:"funcao_id"`            // policiamentos_funcoes.id
	PostoGraduacaoID   *int   `json:"posto_graduacao_id"`   // postos_graduacoes.id
	Posto              string `json:"posto"`
	Numeral            string `json:"numeral"`
	NomeGuerra         string `json:"nome_guerra"`
	Equipe             string `json:"equipe"` // "RAIO 01", "VTRA 121"
}

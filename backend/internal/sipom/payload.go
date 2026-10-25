package sipom

// Contrato do envio ao SIPOM — o corpo de POST /api/integracoes/ocorrencias/
// receber, como o manual do SIPOM o define ("API de integração SIPOM —
// Referência", 2026-10-19). Todos os ids são os das tabelas do SIPOM; só vai
// o que o SIPOM lê (os nomes ao lado dos ids ele ignora, e conteúdo igual
// precisa dar "sem_alteracao" no reenvio).
//
// A ocorrência é identificada por origem.id; cada item leva o id estável do
// Tevunah (o uuid da linha), pelo qual o SIPOM sincroniza a atualização:
// novo entra, alterado é trocado, ausente sai.
//
// Versão "5": o contrato do manual. As anteriores ("1" a "4") eram a nossa
// proposta (docs/sipom-integracao.md).
const PayloadVersion = "5"

// Limites do manual (itens por ocorrência e tamanho de foto).
const (
	MaxComposicao    = 30
	MaxProcedimentos = 20
	MaxMateriais     = 100
	MaxFotos         = 10
	MaxEnvolvidos    = 50
	MaxFotoBytes     = 5 << 20
	MaxNumeroOcorr   = 45
)

// Payload é uma ocorrência completa, num POST só (o SIPOM grava tudo ou
// nada).
type Payload struct {
	Origem                PayloadOrigem         `json:"origem"`
	Ocorrencia            PayloadOcorrencia     `json:"ocorrencia"`
	Historico             string                `json:"historico,omitempty"`
	HistoricoInteligencia string                `json:"historico_inteligencia,omitempty"`
	Composicao            []PayloadComposicao   `json:"composicao"`
	Procedimentos         []PayloadProcedimento `json:"procedimentos"`
	Materiais             []PayloadMaterial     `json:"materiais"`
	Fotos                 []PayloadFoto         `json:"fotos"`
	Envolvidos            []PayloadEnvolvido    `json:"envolvidos"`
}

// PayloadOrigem: origem.id (uuid da ocorrência no Tevunah) é a chave de
// idempotência.
type PayloadOrigem struct {
	ID string `json:"id"`
}

// PayloadOcorrencia é o cabeçalho.
type PayloadOcorrencia struct {
	// Data e hora do fato, ISO 8601 com o fuso do Ceará (-03:00).
	DataHora       string `json:"data_hora"`
	NaturezaFatoID int    `json:"natureza_fato_id"` // natureza_fatos.id
	// Área da unidade militar do local do fato (companhias.id, qualquer
	// batalhão).
	UnidadeMilitarID int `json:"unidade_militar_id"`
	// OPM da equipe (companhias.id); tem de estar no escopo do cliente.
	OPMMilitarID     int             `json:"opm_militar_id"`
	Endereco         PayloadEndereco `json:"endereco"`
	NumeroOcorrencia string          `json:"numero_ocorrencia,omitempty"` // ficha CIOPS, até 45
	Viatura          string          `json:"viatura,omitempty"`
	NumeroHT         string          `json:"numero_ht,omitempty"`
}

// PayloadEndereco: cidade, bairro e coordenada são obrigatórios; o bairro
// tem de ser da cidade. Numeral não numérico o SIPOM deixa vazio (aviso
// numeral_nao_numerico).
type PayloadEndereco struct {
	CidadeID   int     `json:"cidade_id"`
	BairroID   int     `json:"bairro_id"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Logradouro string  `json:"logradouro,omitempty"`
	Numeral    string  `json:"numeral,omitempty"`
}

// PayloadComposicao é um policial da guarnição. Matrícula fora dos
// servidores do SIPOM: o item é ignorado, com aviso matricula_nao_encontrada.
type PayloadComposicao struct {
	ID                 string `json:"id"`
	Matricula          string `json:"matricula"`
	PoliciamentoTipoID int    `json:"policiamento_tipo_id"` // policiamentos_tipos.id
	FuncaoID           int    `json:"funcao_id"`            // policiamentos_funcoes.id
}

// Repartições do procedimento.
const (
	ReparticaoPM             = 1
	ReparticaoPoliciaCivil   = 2
	ReparticaoPoliciaFederal = 3
)

// PayloadProcedimento: o relatório operacional só traz procedimento da
// Polícia Civil (repartição 2: delegacia e delegado do catálogo).
type PayloadProcedimento struct {
	ID             string `json:"id"`
	ProcedimentoID int    `json:"procedimento_id"` // procedimentos.id
	Numero         string `json:"numero"`
	Ano            string `json:"ano"` // AAAA
	Reparticao     int    `json:"reparticao"`
	DelegaciaID    int    `json:"delegacia_id,omitempty"` // delegacias.id
	DelegadoID     *int   `json:"delegado_id,omitempty"`  // delegados.id (opcional)
}

// Tipos de material (material_tipos.id).
const (
	MaterialArma     = 1
	MaterialMunicao  = 2
	MaterialDroga    = 3
	MaterialVeiculo  = 4
	MaterialCelular  = 5
	MaterialDinheiro = 6
	MaterialOutros   = 7
)

// PayloadMaterial é um item da aba Materiais: o tipo decide os campos.
type PayloadMaterial struct {
	ID     string `json:"id"`
	TipoID int    `json:"tipo_id"` // material_tipos.id

	// Arma: tipo obrigatório; marca, calibre, número e descrição opcionais.
	ArmaTipoID    *int   `json:"arma_tipo_id,omitempty"`
	ArmaMarcaID   *int   `json:"arma_marca_id,omitempty"`
	ArmaCalibreID *int   `json:"arma_calibre_id,omitempty"`
	Numero        string `json:"numero,omitempty"`
	Descricao     string `json:"descricao,omitempty"`

	// Droga.
	DrogaTipoID *int `json:"droga_tipo_id,omitempty"`

	// Quantidade: unidades (arma), na unidade da droga, ou valor em R$.
	Quantidade *float64 `json:"quantidade,omitempty"`

	// Veículo: placa já cadastrada no SIPOM é só vinculada; placa nova pede
	// os códigos DENATRAN e o ano de fabricação.
	Placa             string `json:"placa,omitempty"`
	Situacao          *int   `json:"situacao,omitempty"` // 1 apreendido, 2 recuperado
	CodigoTipo        *int   `json:"codigo_tipo,omitempty"`
	CodigoCor         *int   `json:"codigo_cor,omitempty"`
	CodigoMarcaModelo *int   `json:"codigo_marca_modelo,omitempty"`
	AnoFabricacao     *int   `json:"ano_fabricacao,omitempty"`
	AnoModelo         *int   `json:"ano_modelo,omitempty"`
}

// PayloadFoto é uma foto da ocorrência (aba Fotos). JPG, PNG ou WebP, até
// 5 MB; o SIPOM confere o tipo pelo conteúdo e regrava em JPG.
type PayloadFoto struct {
	ID      string `json:"id"`
	Base64  string `json:"base64"`
	Legenda string `json:"legenda,omitempty"`
}

// PayloadEnvolvido: pessoa nil = não identificado (com sexo no item, se
// souber).
type PayloadEnvolvido struct {
	ID        string         `json:"id"`
	VinculoID int            `json:"vinculo_id"`
	Morte     bool           `json:"morte"`
	Pessoa    *PayloadPessoa `json:"pessoa"`
	Sexo      int            `json:"sexo,omitempty"` // só sem pessoa
}

// PayloadPessoa: encontrada por CPF, ou por nome + mãe + nascimento, é só
// vinculada no SIPOM (e só os campos vazios lá são preenchidos).
type PayloadPessoa struct {
	Nome       string           `json:"nome"` // 3 a 125 caracteres
	Sexo       int              `json:"sexo"` // 1 masculino, 2 feminino
	CPF        string           `json:"cpf,omitempty"`
	Nascimento string           `json:"nascimento,omitempty"` // AAAA-MM-DD
	Mae        string           `json:"mae,omitempty"`
	Foto       *PayloadFotoMime `json:"foto,omitempty"`
}

// PayloadFotoMime é a foto da pessoa: {mime, base64}.
type PayloadFotoMime struct {
	Mime   string `json:"mime"`
	Base64 string `json:"base64"`
}

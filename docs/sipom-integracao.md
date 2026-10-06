# Integração Tevunah → SIPOM — proposta de contrato (v1)

**De:** SAI/2º BPRAIO — Tevunah (sistema de inteligência da agência)
**Para:** desenvolvimento do SIPOM (sipom.pm.ce.gov.br)
**Situação:** proposta para avaliação. Nada é enviado ao SIPOM até a aprovação deste contrato e a liberação do canal.

---

## 1. Objetivo

O Tevunah importa o Relatório Diário de Ocorrências do CPRAIO e mantém as ocorrências do 2º BPRAIO com envolvidos qualificados. Hoje esse mesmo conteúdo é redigitado no SIPOM pelo formulário "Criando nova ocorrência" e pelas abas da ocorrência.

A proposta é um **POST único** por ocorrência, que cria no SIPOM a ocorrência já com histórico, envolvidos e composição. Todos os códigos (natureza, unidade, OPM, cidade, bairro, policiamento, função, posto) usam os **ids das tabelas do próprio SIPOM**, a partir do dump de referência entregue em 02/10/2026.

### Escopo

| Fase | Conteúdo |
|---|---|
| **1 (esta proposta)** | Cabeçalho da ocorrência, histórico, envolvidos (com foto) e composição |
| **2 (implementada; contrato "2")** | Procedimento (delegacia e delegado do dump de 05/10/2026) e materiais apreendidos (armas, drogas, veículos) — ver `sipom-materiais.md` |
| 3 (a combinar) | Munição, celulares, dinheiro e outros materiais; fotos da ocorrência; histórico de inteligência |

---

## 2. Endpoint proposto

```
POST /api/integracao/v1/ocorrencias
Content-Type: application/json; charset=utf-8
Authorization: Bearer <token da integração>
Idempotency-Key: <origem.id>
X-Request-Id: <uuid por requisição, para rastreio nos logs dos dois lados>
```

- **Transporte:** HTTPS. Se for possível, também uma lista de IPs liberados para o servidor do Tevunah.
- **Autenticação:** um token por sistema integrado, emitido e revogável pelo SIPOM. O usuário que aparece como autor no SIPOM fica a critério de vocês: um usuário técnico "TEVUNAH – SAI/2º BPRAIO", ou o analista indicado em `origem`.
- **Atomicidade:** a ocorrência e todos os itens são gravados numa transação. Se qualquer item falhar, nada é gravado.

---

## 3. Idempotência e duplicidade

- `origem.sistema` + `origem.id` identificam o registro no Tevunah. **Reenviar o mesmo `origem.id` não cria outra ocorrência**: o SIPOM responde `200` com a ocorrência já criada. É o que permite repetir com segurança depois de uma falha de rede.
- `numero_ocorrencia` é a ficha CIOPS. Se ela já existir no SIPOM, cadastrada por outro meio, a proposta é responder `409` com o identificador da existente, sem criar duplicata. Ver pergunta 3 da seção 8.

---

## 4. Corpo da requisição

As datas seguem a ISO 8601. `data_hora` vai com o fuso do Ceará (`-03:00`); `nascimento` vai como `YYYY-MM-DD`. Os campos de texto que acompanham um id (`natureza_fato`, `unidade_militar`, `opm_militar`) servem **só para conferência**: o que vale é o id.

### 4.1 `origem`

| Campo | Tipo | Obrig. | Descrição |
|---|---|---|---|
| `sistema` | string | sim | Sempre `"TEVUNAH"` |
| `versao` | string | sim | Versão deste contrato (`"1"`) |
| `unidade` | string | sim | Agência que envia (`"SAI/2º BPRAIO"`) |
| `id` | uuid | sim | Id da ocorrência no Tevunah e chave de idempotência |
| `relatorio_data` | date \| null | não | Data do relatório do CPRAIO de onde a ocorrência veio |

### 4.2 `ocorrencia`: campos do formulário "Criando nova ocorrência"

| Campo | Tipo | Obrig. | Correspondência no SIPOM |
|---|---|---|---|
| `natureza_fato_id` | int | sim | `natureza_fatos.id` (campo `natureza_fato`) |
| `natureza_fato` | string | — | `natureza_fatos.nome`, só conferência |
| `data_hora` | datetime | sim | campo `data_hora` |
| `unidade_militar_id` | int | sim | `companhias.id`: área da unidade militar do local do fato (campo `unidadeMilitar`), resolvida por `companhia_atuacao` |
| `unidade_militar` | string | — | `companhias.abreviado`, só conferência |
| `opm_militar_id` | int | sim | `companhias.id`: OPM que atendeu (campo `opmMilitar`). No 2º BPRAIO: 92 = 1ªCIA, 93 = 2ªCIA |
| `opm_militar` | string | — | `companhias.abreviado`, só conferência |
| `endereco.logradouro` | string | sim | campo `end` |
| `endereco.numeral` | string | não | campo `numeral` (pode trazer complemento: `"335 - Casa 39a"`, ou `"S/N"`) |
| `endereco.bairro` | string | sim | campo `bairro` (texto) |
| `endereco.bairro_id` | int \| null | não | `bairro.id` quando o bairro existe no catálogo |
| `endereco.cidade` | string | sim | campo `cidade` (texto) |
| `endereco.cidade_id` | int | sim | `cidade.id` |
| `endereco.cidade_ibge` | int | sim | `cidade.codigo` (IBGE) |
| `endereco.latitude` / `longitude` | number \| null | não | campos ocultos `latitude`/`longitude`. O relatório não traz coordenadas; hoje vão `null` |
| `numero_ocorrencia` | string | sim | campo `numero_ocorrencia` (ficha CIOPS) |
| `viatura` | string \| null | não | campo `viatura_ocorrencia` (o relatório não traz; vai `null`) |
| `numero_ht` | string \| null | não | campo `numero_ht` (idem) |
| `participacao_inteligencia` | bool | sim | Houve participação da inteligência (SAI) na ocorrência. **Sem campo equivalente hoje**: ver pergunta 5 |

### 4.3 `historico` e `historico_inteligencia`

| Campo | Tipo | Obrig. | Correspondência |
|---|---|---|---|
| `historico` | string | sim | Aba **Histórico**: texto do relatório, sem alteração |
| `historico_inteligencia` | string \| null | não | Aba **Inteligência**. Reservado: vai `null` na fase 1 |

### 4.4 `envolvidos[]`: modal "Adicionar Pessoa à Ocorrência"

| Campo | Tipo | Obrig. | Correspondência |
|---|---|---|---|
| `vinculo` | enum | sim | Select "Pessoas Envolvidas" (abaixo) |
| `nome` | string \| null | não | campo `nome` (null para não identificados) |
| `cpf` | string \| null | não | campo `cpf`, 11 dígitos sem máscara |
| `sexo` | int \| null | não | campo `sexo`: 1 Masculino, 2 Feminino |
| `nascimento` | date \| null | não | campo `nascimento` |
| `mae` | string \| null | não | campo `mae` |
| `morte` | bool | sim | campo `morte` (0/1). Óbito **nesta** ocorrência |
| `foto` | object \| null | não | campo `file_image`: `{ "mime": "image/jpeg" \| "image/png", "base64": "..." }`, até 5 MB |

**Códigos de `vinculo`.** No formulário, o valor das opções é um token que muda a cada carregamento, por isso o contrato usa códigos estáveis:

| Código | Opção no SIPOM |
|---|---|
| `VITIMA` | Vitima |
| `INFRATOR` | Infrator |
| `TESTEMUNHA` | Testemunha |
| `VITIMA_NAO_IDENTIFICADA` | Vitma - Não Identificada |
| `INFRATOR_NAO_IDENTIFICADO` | Infrator - Não identificado |

**Pessoa já cadastrada no SIPOM.** O esperado é o mesmo comportamento do modal: localizar pelo CPF, reaproveitar o cadastro e, se a pessoa já tiver foto principal, guardar a enviada como foto extra.

### 4.5 `composicao[]`: aba Composições

| Campo | Tipo | Obrig. | Correspondência |
|---|---|---|---|
| `matricula` | string | sim | Matrícula do policial (o SIPOM localiza o servidor, como em `ocorrencias-consulta-matricula-servidor`) |
| `policiamento_tipo_id` | int | sim | `policiamentos_tipos.id` |
| `funcao_id` | int | sim | `policiamentos_funcoes.id`, sempre uma combinação válida de `policiamentos_tipos_funcoes` |
| `posto_graduacao_id` | int \| null | não | `postos_graduacoes.id` |
| `posto`, `numeral`, `nome_guerra` | string | — | Conferência |
| `equipe` | string | — | Equipe do relatório (`"RAIO 01"`, `"VTRA 121"`) |

Regra de origem, adotada pelo batalhão: equipe **VTRA** → Motorizado (6), com PM1 Comandante, PM2 Motorista e os demais Patrulheiros. Equipe **RAIO** → Motopatrulhamento (7), com PM1 Comandante, PM2 Subcomandante, PM3 3 homem, PM4 Garupa e PM5 5 Homem.

---

## 5. Respostas

Envelope: `{ "success": bool, "data": ..., "message": string, "errors": [...] }`.

| Status | Quando | `data` |
|---|---|---|
| `201 Created` | Ocorrência criada | `{ "id": "...", "identificador": "2026D7B16BC7", "url": "https://sipom.pm.ce.gov.br/ocorrencias/ocorrencias-exibir/..." }` |
| `200 OK` | `origem.id` já recebido (reenvio) | O mesmo de quando foi criada |
| `400 Bad Request` | JSON inválido ou campo obrigatório ausente | — (`errors`: `[{ "campo": "ocorrencia.data_hora", "mensagem": "..." }]`) |
| `401` / `403` | Token ausente, inválido ou sem permissão | — |
| `409 Conflict` | `numero_ocorrencia` já existe, criada por outro meio | `{ "id": "...", "identificador": "..." }` da existente |
| `422 Unprocessable Entity` | Id inexistente ou desativado (natureza, companhia, função…) | — (`errors` por campo) |
| `5xx` | Falha do SIPOM | O Tevunah tenta de novo com intervalo crescente, com a mesma `Idempotency-Key` |

---

## 6. Exemplo (dados fictícios)

Gerado pelo próprio montador do Tevunah, com o catálogo real e uma ocorrência inventada:

```json
{
  "origem": {
    "sistema": "TEVUNAH",
    "versao": "1",
    "unidade": "SAI/2º BPRAIO",
    "id": "3f6d2a9e-0000-4000-8000-000000000000",
    "relatorio_data": "2026-09-26"
  },
  "ocorrencia": {
    "natureza_fato_id": 10,
    "natureza_fato": "TRAFICO DE ENTORPECENTES",
    "data_hora": "2026-09-26T14:30:00-03:00",
    "unidade_militar_id": 174,
    "unidade_militar": "2ªCIA/26ºBPM",
    "opm_militar_id": 92,
    "opm_militar": "1ªCIA/2ºBPRAIO",
    "endereco": {
      "logradouro": "Rua Exemplo",
      "numeral": "100",
      "bairro": "PARQUE GUADALAJARA",
      "bairro_id": 390,
      "cidade": "CAUCAIA",
      "cidade_id": 919,
      "cidade_ibge": 2303709,
      "latitude": null,
      "longitude": null
    },
    "numero_ocorrencia": "M20260700000",
    "viatura": null,
    "numero_ht": null,
    "participacao_inteligencia": true
  },
  "historico": "Texto do histórico como registrado no relatório (exemplo).",
  "historico_inteligencia": null,
  "envolvidos": [
    {
      "vinculo": "INFRATOR",
      "nome": "FULANO DE TAL DA SILVA",
      "cpf": "00000000191",
      "sexo": 1,
      "nascimento": "1999-03-14",
      "mae": "MARIA DE TAL DA SILVA",
      "morte": false,
      "foto": {
        "mime": "image/jpeg",
        "base64": "/9j/4AAQSkZJRgABAQ…(conteúdo da imagem em base64)"
      }
    },
    {
      "vinculo": "VITIMA_NAO_IDENTIFICADA",
      "nome": null,
      "cpf": null,
      "sexo": null,
      "nascimento": null,
      "mae": null,
      "morte": false,
      "foto": null
    },
    {
      "vinculo": "TESTEMUNHA",
      "nome": "BELTRANO DE TAL",
      "cpf": null,
      "sexo": null,
      "nascimento": null,
      "mae": null,
      "morte": false,
      "foto": null
    }
  ],
  "composicao": [
    { "matricula": "30000001", "policiamento_tipo_id": 7, "funcao_id": 1, "posto_graduacao_id": 3, "posto": "3ºSGT", "numeral": "20001", "nome_guerra": "EXEMPLO A", "equipe": "RAIO 01" },
    { "matricula": "30000002", "policiamento_tipo_id": 7, "funcao_id": 2, "posto_graduacao_id": 2, "posto": "CB", "numeral": "20002", "nome_guerra": "EXEMPLO B", "equipe": "RAIO 01" },
    { "matricula": "30000003", "policiamento_tipo_id": 7, "funcao_id": 3, "posto_graduacao_id": 2, "posto": "CB", "numeral": "20003", "nome_guerra": "EXEMPLO C", "equipe": "RAIO 01" },
    { "matricula": "30000004", "policiamento_tipo_id": 7, "funcao_id": 4, "posto_graduacao_id": 1, "posto": "SD", "numeral": "20004", "nome_guerra": "EXEMPLO D", "equipe": "RAIO 01" }
  ]
}
```

---

## 7. O que o Tevunah garante antes de enviar

- Só envia ocorrência sem pendência: natureza, data e hora, área, endereço, OPM e ficha CIOPS resolvidos. Casos ambíguos (local em mais de uma área, ficha com duas equipes, natureza sugerida) são decididos por um analista antes.
- Todo id enviado existe no catálogo do SIPOM; companhias desativadas não são usadas.
- Fotos só de dossiês dentro do nível de acesso de quem envia. Todo envio fica na auditoria do Tevunah.

---

## 8. Perguntas para o desenvolvimento do SIPOM

1. **Canal:** o formato acima (POST único, JSON, token Bearer) serve? Há preferência por outro (por exemplo, `multipart` para fotos)?
2. **Autoria:** quem aparece como autor da ocorrência criada por integração?
3. **Ficha CIOPS já cadastrada:** devolver `409` e não criar, ou permitir complementar a existente?
4. **Correções:** depois de criada, a ocorrência pode ser atualizada pela integração (por exemplo, `PUT` com o mesmo `origem.id`), ou correções são só pela tela?
5. **Participação da inteligência:** existe (ou pode existir) campo para `participacao_inteligencia`? Ou o equivalente seria usar a aba Inteligência, ou um policiamento do tipo 8 – Inteligência na composição?
6. **Ambiente de homologação:** há uma instância de testes para validarmos os envios sem gravar em produção?
7. **Fase 2:** para materiais e procedimento, os catálogos de **drogas**, **marcas de celular**, **delegacias** e **delegados** não vieram no dump. Podem enviá-los, ou devemos usar os textos/códigos que aparecem no formulário?

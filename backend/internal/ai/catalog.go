// Package ai guarda a configuração do assistente de IA de cada usuário
// (chave da API Anthropic cifrada, modelo escolhido, perfil de uso para a
// estimativa de custo) e fala com a API para validar a chave.
//
// As features de IA que vierem depois pegam a chave e o modelo do usuário
// logado por Repo.Credentials — nunca há chave global do sistema.
package ai

// Model é um modelo oferecido na tela, com o preço de tabela da API
// Anthropic (US$ por milhão de tokens, sem cache nem lote).
type Model struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Line        string  `json:"line"`
	InputUSD    float64 `json:"input_usd_per_mtok"`
	OutputUSD   float64 `json:"output_usd_per_mtok"`
	Context     string  `json:"context"`
	Description string  `json:"description"`
	Recommended bool    `json:"recommended"`
}

// PricesAsOf é a data da tabela de preços abaixo. Preço muda: conferir em
// https://www.anthropic.com/pricing ao atualizar o catálogo.
const PricesAsOf = "2026-11-03"

// DefaultModel é o modelo pré-selecionado para quem ainda não escolheu.
const DefaultModel = "claude-haiku-5-5"

// Catalog: geração atual de cada linha, do mais capaz ao mais barato.
var Catalog = []Model{
	{
		ID: "claude-fable-5-2", Name: "Claude Fable 5.2", Line: "fable",
		InputUSD: 8, OutputUSD: 40, Context: "1M",
		Description: "O mais capaz: raciocínio mais exigente e tarefas longas. Respostas mais demoradas e o maior custo.",
	},
	{
		ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Line: "opus",
		InputUSD: 4, OutputUSD: 20, Context: "1M",
		Description: "Análise e apoio à decisão com alta qualidade, a um custo intermediário.",
	},
	{
		ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Line: "sonnet",
		InputUSD: 2, OutputUSD: 10, Context: "1M",
		Description: "Equilíbrio entre velocidade, qualidade e custo para o uso diário.",
	},
	{
		ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Line: "haiku",
		InputUSD: 0.10, OutputUSD: 0.50, Context: "1M",
		Description: "Padrão: o mais rápido e barato — triagem, classificação e extração em volume. Preço vale para pedidos de até 100 mil tokens.",
		Recommended: true,
	},
}

// FindModel devolve o modelo do catálogo, ou nil.
func FindModel(id string) *Model {
	for i := range Catalog {
		if Catalog[i].ID == id {
			return &Catalog[i]
		}
	}
	return nil
}

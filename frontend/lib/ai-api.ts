import { api } from "./api";

// Assistente de IA — configuração do próprio usuário. A chave da API
// Anthropic só sobe (PUT) e nunca volta: a tela recebe a dica "sk-ant-…a1B2".

export type AIModel = {
  id: string;
  name: string;
  line: string;
  input_usd_per_mtok: number;
  output_usd_per_mtok: number;
  cache_read_usd_per_mtok: number;
  context: string;
  description: string;
  recommended: boolean;
};

/** Soma do uso num período (custo calculado no registro de cada chamada). */
export type AIPeriod = {
  cost_usd: number;
  requests: number;
  input_tokens: number;
  output_tokens: number;
};

export type AISettings = {
  /** O servidor tem o cofre de segredos (APP_SECRETS_KEY) para guardar a chave. */
  secrets_ready: boolean;
  configured: boolean;
  key_hint?: string;
  key_verified_at?: string;
  /** Modelos que a chave enxerga (na última verificação). */
  available_models: string[];
  model: string;
  /** Gasto do mês e do ano correntes (calendário da agência). */
  spending: { month: AIPeriod; year: AIPeriod };
  catalog: AIModel[];
  prices_as_of: string;
};

export function getAISettings() {
  return api<AISettings>("/api/me/ai");
}

export function saveAIKey(apiKey: string) {
  return api<AISettings>("/api/me/ai/key", {
    method: "PUT",
    body: JSON.stringify({ api_key: apiKey }),
  });
}

export function verifyAIKey() {
  return api<AISettings>("/api/me/ai/key/verify", { method: "POST" });
}

export function deleteAIKey() {
  return api<AISettings>("/api/me/ai/key", { method: "DELETE" });
}

export function saveAIModel(model: string) {
  return api<AISettings>("/api/me/ai/settings", {
    method: "PUT",
    body: JSON.stringify({ model }),
  });
}

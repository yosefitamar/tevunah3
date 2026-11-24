import { api } from "./api";

// Assistente de IA — configuração do próprio usuário. A chave da API
// Anthropic só sobe (PUT) e nunca volta: a tela recebe a dica "sk-ant-…a1B2".

export type AIModel = {
  id: string;
  name: string;
  line: string;
  input_usd_per_mtok: number;
  output_usd_per_mtok: number;
  context: string;
  description: string;
  recommended: boolean;
};

export type AIUsageProfile = {
  requests_per_day: number;
  work_days_per_month: number;
  input_tokens_per_request: number;
  output_tokens_per_request: number;
  usd_brl: number;
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
  usage: AIUsageProfile;
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

export function saveAISettings(model: string, usage: AIUsageProfile) {
  return api<AISettings>("/api/me/ai/settings", {
    method: "PUT",
    body: JSON.stringify({ model, usage }),
  });
}

/** Custo estimado em US$ de um perfil de uso num modelo (preço de tabela). */
export function estimateUSD(m: AIModel, u: AIUsageProfile) {
  const perRequest =
    (u.input_tokens_per_request * m.input_usd_per_mtok +
      u.output_tokens_per_request * m.output_usd_per_mtok) /
    1_000_000;
  const monthly = perRequest * u.requests_per_day * u.work_days_per_month;
  return { perRequest, monthly, annual: monthly * 12 };
}

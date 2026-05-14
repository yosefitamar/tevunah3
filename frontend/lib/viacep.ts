// Wrapper sobre a API pública do ViaCEP (https://viacep.com.br).
// Aceita CORS, então o fetch sai direto do browser. CEP precisa estar no
// formato 8 dígitos (com ou sem hífen — a função normaliza).

export type ViaCEPResult = {
  cep: string;
  logradouro: string;
  complemento: string;
  bairro: string;
  localidade: string; // cidade
  uf: string;
  erro?: boolean;
};

/**
 * Busca o endereço pelo CEP. Retorna null para CEPs inválidos (não-8-dígitos)
 * ou inexistentes (ViaCEP devolve { erro: true }).
 */
export async function fetchAddressByCEP(cep: string): Promise<ViaCEPResult | null> {
  const digits = cep.replace(/\D/g, "");
  if (digits.length !== 8) return null;
  try {
    const res = await fetch(`https://viacep.com.br/ws/${digits}/json/`);
    if (!res.ok) return null;
    const body = (await res.json()) as ViaCEPResult;
    if (body.erro) return null;
    return body;
  } catch {
    return null;
  }
}

/** Aplica máscara 99999-999 sobre uma string de dígitos. */
export function formatCEP(s: string): string {
  const d = s.replace(/\D/g, "").slice(0, 8);
  if (d.length <= 5) return d;
  return `${d.slice(0, 5)}-${d.slice(5)}`;
}

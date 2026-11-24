// Versão que o servidor está servindo agora. A aba aberta compara com a dela
// (NEXT_PUBLIC_BUILD_ID, gravado no código no build) para saber se houve deploy.
export const dynamic = "force-dynamic";

export function GET() {
  return Response.json(
    { build: process.env.NEXT_PUBLIC_BUILD_ID ?? "" },
    { headers: { "Cache-Control": "no-store, max-age=0" } },
  );
}

// Logotipo do Tevunah — versão animada (camada base estática + camada de fluxo
// com stroke-dasharray rotando + breath no núcleo). Usa currentColor para
// herdar a cor do container, então acompanha a paleta ativa.
//
// As keyframes ficam em frontend/app/globals.css (.tv-edge, .tv-core).
export default function TevunahLogo({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 64 64"
      fill="none"
      role="img"
      aria-label="Tevunah"
    >
      <title>Tevunah</title>

      {/* Camada base — arestas estáticas com baixa opacidade */}
      <g stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" fill="none" opacity="0.18">
        <line x1="14" y1="20" x2="32" y2="32" />
        <line x1="50" y1="20" x2="32" y2="32" />
        <line x1="14" y1="44" x2="32" y2="32" />
        <line x1="50" y1="44" x2="32" y2="32" />
        <line x1="32" y1="12" x2="32" y2="32" />
        <line x1="14" y1="20" x2="32" y2="12" />
        <line x1="50" y1="20" x2="32" y2="12" />
        <line x1="14" y1="20" x2="14" y2="44" />
        <line x1="50" y1="20" x2="50" y2="44" />
      </g>

      {/* Camada de fluxo — tracejado em deslocamento contínuo */}
      <g stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" fill="none">
        <line className="tv-edge" x1="14" y1="20" x2="32" y2="32" />
        <line className="tv-edge" x1="50" y1="20" x2="32" y2="32" />
        <line className="tv-edge" x1="14" y1="44" x2="32" y2="32" />
        <line className="tv-edge" x1="50" y1="44" x2="32" y2="32" />
        <line className="tv-edge" x1="32" y1="12" x2="32" y2="32" />
        <line className="tv-edge" x1="14" y1="20" x2="32" y2="12" />
        <line className="tv-edge" x1="50" y1="20" x2="32" y2="12" />
        <line className="tv-edge" x1="14" y1="20" x2="14" y2="44" />
        <line className="tv-edge" x1="50" y1="20" x2="50" y2="44" />
      </g>

      {/* Nós */}
      <g fill="currentColor">
        <circle cx="14" cy="20" r="2.6" />
        <circle cx="50" cy="20" r="2.6" />
        <circle cx="14" cy="44" r="2.6" />
        <circle cx="50" cy="44" r="2.6" />
        <circle cx="32" cy="12" r="2.6" />
        <circle className="tv-core" cx="32" cy="32" r="4.5" />
      </g>
    </svg>
  );
}

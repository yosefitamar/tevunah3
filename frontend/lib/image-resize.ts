// Redução de foto no navegador, só quando o arquivo não cabe no limite do
// servidor (5 MiB). A foto que cabe sobe como veio, sem recompressão: ela é
// o registro da ocorrência e vai inteira ao SIPOM. Foto de celular acima do
// limite é regravada em JPEG com até 1920 px no maior lado.

const MAX_SIDE = 1920;
const QUALITY = 0.9;
/** Limite de upload do servidor (photoMaxBytes). */
export const PHOTO_MAX_BYTES = 5 << 20;

/**
 * Devolve a imagem pronta para o upload. Dentro do limite, o arquivo original,
 * intacto. Acima dele, um JPEG reduzido. Se o navegador não conseguir
 * decodificar, volta o original — o servidor recusa com a mensagem certa.
 */
export async function shrinkImage(file: File): Promise<File> {
  if (file.size <= PHOTO_MAX_BYTES || !/^image\/(jpeg|png)$/.test(file.type)) return file;
  let bmp: ImageBitmap;
  try {
    bmp = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    return file;
  }
  try {
    const scale = Math.min(1, MAX_SIDE / Math.max(bmp.width, bmp.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.round(bmp.width * scale);
    canvas.height = Math.round(bmp.height * scale);
    const ctx = canvas.getContext("2d");
    if (!ctx) return file;
    // Fundo branco: PNG com transparência viraria preto no JPEG.
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(bmp, 0, 0, canvas.width, canvas.height);
    const blob = await new Promise<Blob | null>((ok) => canvas.toBlob(ok, "image/jpeg", QUALITY));
    if (!blob || blob.size >= file.size) return file;
    return new File([blob], file.name.replace(/\.[^.]+$/, "") + ".jpg", { type: "image/jpeg" });
  } finally {
    bmp.close();
  }
}

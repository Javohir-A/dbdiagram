// html-to-image and jsPDF are loaded on first export, not at startup.

const skip = (node: Node) => {
  const c = (node as HTMLElement).classList
  // The minimap and the floating bars are chrome, not part of the picture.
  return !(c?.contains('react-flow__minimap') || c?.contains('react-flow__controls') || c?.contains('react-flow__panel'))
}

export async function renderPng(el: HTMLElement): Promise<string> {
  const { toPng } = await import('html-to-image')
  return toPng(el, { backgroundColor: '#fff', pixelRatio: 2, filter: skip })
}

/** Hands a Blob or a ready URL to the browser as a file download. */
export function download(filename: string, content: string | Blob) {
  const blob = typeof content === 'string' ? new Blob([content], { type: 'text/plain' }) : content
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function downloadUrl(filename: string, url: string) {
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
}

export async function exportPdf(el: HTMLElement, filename: string) {
  const [url, { jsPDF }] = await Promise.all([renderPng(el), import('jspdf')])
  const img = new Image()
  img.src = url
  await img.decode()
  const w = img.width / 2
  const h = img.height / 2
  const pdf = new jsPDF({ orientation: w >= h ? 'l' : 'p', unit: 'px', format: [w, h] })
  pdf.addImage(url, 'PNG', 0, 0, w, h)
  pdf.save(filename)
}

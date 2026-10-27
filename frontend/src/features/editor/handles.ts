/** Handles are addressed by column index, never by name: names can repeat or contain anything. */
export const handleId = (columnIndex: number, side: 'l' | 'r') => `col-${columnIndex}-${side}`

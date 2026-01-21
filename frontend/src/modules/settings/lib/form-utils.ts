export const normalizeNumber = (value: string) => {
  const nextValue = Number(value)
  return Number.isNaN(nextValue) ? 0 : nextValue
}

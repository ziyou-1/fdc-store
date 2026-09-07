type PointsSource = { points_price?: unknown } | null | undefined

export function pointsPriceOf(product?: PointsSource, sku?: PointsSource): number {
  const raw = sku?.points_price ?? product?.points_price ?? 0
  const n = Math.trunc(Number(raw))
  return Number.isFinite(n) && n > 0 ? n : 0
}

export function canRedeemWithPoints(product?: PointsSource, sku?: PointsSource): boolean {
  return pointsPriceOf(product, sku) > 0
}

export function redeemPointsCost(product?: PointsSource, sku?: PointsSource, quantity?: unknown): number {
  const unit = pointsPriceOf(product, sku)
  const qty = Math.trunc(Number(quantity))
  if (unit <= 0 || !Number.isFinite(qty) || qty <= 0) return 0
  return unit * qty
}

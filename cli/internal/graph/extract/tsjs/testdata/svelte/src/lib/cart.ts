export interface Line {
  sku: string;
  qty: number;
}

export function addLine(lines: Line[], sku: string): Line[] {
  return [...lines, { sku, qty: 1 }];
}

export function totalQty(lines: Line[]): number {
  return lines.reduce((sum, line) => sum + line.qty, 0);
}

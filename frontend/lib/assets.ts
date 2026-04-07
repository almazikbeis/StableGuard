"use client";

export type AssetType = "stable" | "volatile";

export interface AssetMeta {
  symbol: string;
  index: number;
  name: string;
  assetType: AssetType;
  decimals: number;
  color: string;
  mainnetMint: string;
}

export const ASSET_OPTIONS: AssetMeta[] = [
  {
    symbol: "USDC",
    index: 0,
    name: "USD Coin",
    assetType: "stable",
    decimals: 6,
    color: "#2563eb",
    mainnetMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  },
  {
    symbol: "USDT",
    index: 1,
    name: "Tether",
    assetType: "stable",
    decimals: 6,
    color: "#16a34a",
    mainnetMint: "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
  },
  {
    symbol: "ETH",
    index: 2,
    name: "Ethereum",
    assetType: "volatile",
    decimals: 6,
    color: "#8b5cf6",
    mainnetMint: "7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs",
  },
  {
    symbol: "SOL",
    index: 3,
    name: "Solana",
    assetType: "volatile",
    decimals: 9,
    color: "#06b6d4",
    mainnetMint: "So11111111111111111111111111111111111111112",
  },
  {
    symbol: "BTC",
    index: 4,
    name: "Bitcoin",
    assetType: "volatile",
    decimals: 6,
    color: "#f97316",
    mainnetMint: "cbbN1qK6s2KQvsJY6b3iH4k6o1YwJw3pLxP8G8mGiqN",
  },
  {
    symbol: "DAI",
    index: 5,
    name: "Dai",
    assetType: "stable",
    decimals: 6,
    color: "#d97706",
    mainnetMint: "EjmyN6qo3JxB6o2gA1LsgWn6CmvNfM5qRjS8Vt3M5wV",
  },
  {
    symbol: "PYUSD",
    index: 6,
    name: "PayPal USD",
    assetType: "stable",
    decimals: 6,
    color: "#9333ea",
    mainnetMint: "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo",
  },
];

export const VOLATILE_SYMBOLS = new Set(
  ASSET_OPTIONS.filter((asset) => asset.assetType === "volatile").map((asset) => asset.symbol),
);

export const STABLE_SYMBOLS = new Set(
  ASSET_OPTIONS.filter((asset) => asset.assetType === "stable").map((asset) => asset.symbol),
);

export function getAssetMeta(symbol: string): AssetMeta | undefined {
  return ASSET_OPTIONS.find((asset) => asset.symbol === symbol);
}

export function isStableSymbol(symbol: string): boolean {
  return STABLE_SYMBOLS.has(symbol);
}

export function isVolatileSymbol(symbol: string): boolean {
  return VOLATILE_SYMBOLS.has(symbol);
}

export function defaultCounterpartySymbol(symbol: string): string {
  if (symbol === "USDC") return "USDT";
  if (isStableSymbol(symbol)) return "USDC";
  return "USDC";
}

import type { Metadata } from "next";
import { Syne, DM_Mono } from "next/font/google";
import "./globals.css";
import { ToastContainer } from "@/components/ToastContainer";
import { WalletProvider } from "@/components/WalletProvider";

const syne = Syne({
  subsets: ["latin"],
  variable: "--font-syne",
  display: "swap",
});

const dmMono = DM_Mono({
  subsets: ["latin"],
  weight: ["300", "400", "500"],
  variable: "--font-dm-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "StableGuard — AI Crypto Treasury",
  description: "AI-managed multi-asset crypto treasury with on-chain execution on Solana",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`h-full ${syne.variable} ${dmMono.variable}`}>
      <body className="min-h-full">
        <WalletProvider>
          {children}
        </WalletProvider>
        <ToastContainer />
      </body>
    </html>
  );
}

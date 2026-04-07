"use client";

import Image from "next/image";
import { useWallet, useConnection } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";
import { Wallet, LogOut, Copy, Check, ChevronDown } from "lucide-react";
import { useState, useEffect } from "react";
import { LAMPORTS_PER_SOL } from "@solana/web3.js";

export function WalletButton() {
  const { connected, publicKey, disconnect, wallet } = useWallet();
  const { setVisible } = useWalletModal();
  const { connection } = useConnection();
  const [copied, setCopied] = useState(false);
  const [solBalance, setSolBalance] = useState<number | null>(null);
  const [showMenu, setShowMenu] = useState(false);

  useEffect(() => {
    if (!connected || !publicKey) return;

    let cancelled = false;
    connection.getBalance(publicKey).then(lamports => {
      if (!cancelled) {
        setSolBalance(lamports / LAMPORTS_PER_SOL);
      }
    }).catch(() => {
      if (!cancelled) {
        setSolBalance(null);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [connected, publicKey, connection]);

  function copyAddress() {
    if (!publicKey) return;
    navigator.clipboard.writeText(publicKey.toBase58());
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  if (connected && publicKey) {
    const short = `${publicKey.toBase58().slice(0, 4)}…${publicKey.toBase58().slice(-4)}`;
    return (
      <div className="relative">
        <button
          onClick={() => setShowMenu(v => !v)}
          className="flex items-center gap-2 data-chip rounded-full px-3 py-1.5 text-xs font-semibold text-slate-100 hover:bg-white/10 transition-colors"
        >
          {wallet?.adapter.icon && (
            <Image
              src={wallet.adapter.icon}
              alt=""
              width={14}
              height={14}
              unoptimized
              className="w-3.5 h-3.5 rounded-sm flex-shrink-0"
            />
          )}
          <span className="font-mono">{short}</span>
          {solBalance !== null && (
            <span className="text-cyan-300 font-mono">{solBalance.toFixed(2)} SOL</span>
          )}
          <ChevronDown size={10} className="text-slate-500" />
        </button>

        {showMenu && (
          <>
            <div className="fixed inset-0 z-40" onClick={() => setShowMenu(false)} />
            <div className="absolute right-0 top-full mt-2 z-50 w-48 rounded-[16px] border border-white/12 bg-[#0d1928] shadow-2xl p-1.5 space-y-0.5">
              <button
                onClick={() => { copyAddress(); setShowMenu(false); }}
                className="w-full flex items-center gap-2 rounded-xl px-3 py-2 text-xs text-slate-300 hover:bg-white/8 transition-colors"
              >
                {copied ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                {copied ? "Copied!" : "Copy address"}
              </button>
              <div className="border-t border-white/8 my-1" />
              <button
                onClick={() => { disconnect(); setShowMenu(false); }}
                className="w-full flex items-center gap-2 rounded-xl px-3 py-2 text-xs text-red-400 hover:bg-red-400/8 transition-colors"
              >
                <LogOut size={12} />
                Disconnect
              </button>
            </div>
          </>
        )}
      </div>
    );
  }

  return (
    <button
      onClick={() => setVisible(true)}
      className="flex items-center gap-1.5 data-chip rounded-full px-3 py-1.5 text-xs font-semibold text-orange-200 hover:bg-orange-400/15 border border-orange-400/25 transition-colors"
    >
      <Wallet size={12} />
      Connect Wallet
    </button>
  );
}

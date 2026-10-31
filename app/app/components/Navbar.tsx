import React from "react";
import Image from "next/image";
import Link from "next/link";

const Navbar = () => {
  return (
    <nav className="fixed left-0 right-0 top-0 z-50 bg-gradient-to-b from-white/80 via-white/50 to-transparent backdrop-blur-md border-b border-slate-200/40">
      <div className="grid h-16 grid-cols-[1fr_auto_1fr] items-center px-6 md:px-10 max-w-[1400px] mx-auto w-full">

        {/* Logo + wordmark */}
        <Link href="/" className="flex items-center gap-2 select-none">
          <Image
            src="/auvis-final.svg"
            alt="Auvis Logo"
            width={28}
            height={28}
            className="h-7 w-7 shrink-0"
          />
          <span
            className="text-[1.2rem] font-semibold tracking-[-0.03em] text-slate-900"
            style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
          >
            Auvis
          </span>
        </Link>

        {/* Nav links: centered, clean text */}
        <div className="hidden items-center gap-8 md:flex">
          <Link href="/#features" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC]">Features</Link>
          <Link href="/#how-it-works" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC]">How it works</Link>
          <Link href="/#faqs" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC]">FAQ</Link>
          {/* Reports code temporarily commented out
          <Link href="/report" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC] flex items-center gap-1.5">
            <span>Reports</span>
            <span className="rounded-full bg-[#7B62FC]/10 px-1.5 py-0.2 font-mono text-[10px] font-semibold text-[#7B62FC]">Weekly</span>
          </Link>
          */}
        </div>

        {/* Actions */}
        <div className="flex items-center justify-end gap-4">
          <Link
            href="/docs"
            className="hidden text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC] md:block"
          >
            Docs
          </Link>
          <a
            href="https://github.com/cridiv/Auvis"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 rounded-lg bg-[#0B0C11] px-4 py-2 text-sm font-medium text-white transition-all hover:bg-[#7B62FC] active:scale-[0.97]"
          >
            <Image src="/icon/github.svg" alt="" width={15} height={15} className="h-[15px] w-[15px] shrink-0" />
            <span>GitHub</span>
          </a>
        </div>

      </div>
    </nav>
  );
};

export default Navbar;

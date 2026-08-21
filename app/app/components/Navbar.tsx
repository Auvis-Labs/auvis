import React from "react";
import Image from "next/image";

const Navbar = () => {
  return (
    <nav className="fixed left-0 right-0 top-0 z-50 bg-gradient-to-b from-white/70 via-white/40 to-transparent backdrop-blur-md">
      <div className="grid h-16 grid-cols-[1fr_auto_1fr] items-center px-10">

        {/* Logo + wordmark */}
        <a href="/" className="flex items-center gap-2 select-none">
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
        </a>

        {/* Nav links: centered, clean text */}
        <div className="hidden items-center gap-8 md:flex">
          <a href="#features" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC]">Features</a>
          <a href="#how-it-works" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC]">How it works</a>
          <a href="#faqs" className="text-sm font-medium text-slate-600 transition-colors hover:text-[#7B62FC]">FAQ</a>
        </div>

        {/* Actions */}
        <div className="flex items-center justify-end gap-4">
          <a href="/docs" className="hidden text-sm font-medium text-slate-600 transition-colors hover:text-slate-900 md:block">
            Docs
          </a>
          <a
            href="https://github.com/cridiv/auvis"
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

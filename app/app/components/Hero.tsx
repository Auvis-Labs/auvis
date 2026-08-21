"use client";

import React from "react";
import { ArrowRight, Gauge, ShieldCheck, Zap } from "lucide-react";
import Image from "next/image";

const Hero = () => {
  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-white px-6 pb-20 pt-32">

      {/* Background: soft multi-layered purple Aurora glow (#7B62FC) */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 z-0 opacity-80"
        style={{
          background: `
            radial-gradient(ellipse 75% 55% at 50% 10%, rgba(123, 98, 252, 0.16) 0%, transparent 70%),
            radial-gradient(ellipse 50% 40% at 80% 40%, rgba(196, 181, 253, 0.10) 0%, transparent 60%),
            radial-gradient(ellipse 50% 40% at 20% 50%, rgba(123, 98, 252, 0.08) 0%, transparent 60%)
          `,
        }}
      />

      {/* Faint radially-masked dot grid pattern */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 z-0 opacity-25"
        style={{
          backgroundImage: "radial-gradient(circle at 1px 1px, #7B62FC 1px, transparent 0)",
          backgroundSize: "24px 24px",
          maskImage: "radial-gradient(ellipse 80% 70% at 50% 45%, black 30%, transparent 90%)",
          WebkitMaskImage: "radial-gradient(ellipse 80% 70% at 50% 45%, black 30%, transparent 90%)",
        }}
      />

      {/* Content */}
      <div
        className="relative z-10 flex max-w-3xl flex-col items-center text-center"
        style={{ animation: "heroFadeUp 0.7s cubic-bezier(0.22,1,0.36,1) both" }}
      >

        {/* Eyebrow text (pure styled text) */}
        <p
          className="mb-5 text-[0.8rem] font-semibold uppercase tracking-[0.14em] text-[#7B62FC]"
          style={{ fontFamily: "var(--font-dm-sans), sans-serif" }}
        >
          Real-Time Policy &amp; Data Protection
        </p>

        {/* Headline */}
        <h1
          className="max-w-3xl text-[3.25rem] font-bold leading-[1.1] tracking-[-0.03em] text-slate-900 sm:text-[3.75rem]"
          style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
        >
          Govern your data
          <br />
          before it reaches your{" "}
          <span
            style={{
              backgroundImage: "linear-gradient(135deg, #7B62FC 0%, #a78bfa 100%)",
              WebkitBackgroundClip: "text",
              WebkitTextFillColor: "transparent",
              backgroundClip: "text",
            }}
          >
            AI.
          </span>
        </h1>

        {/* Subtext */}
        <p
          className="mt-6 max-w-xl text-[1.05rem] leading-relaxed text-slate-500"
          style={{
            fontFamily: "var(--font-dm-sans), sans-serif",
            animation: "heroFadeUp 0.7s 0.12s cubic-bezier(0.22,1,0.36,1) both",
          }}
        >
          Auvis intercepts ingestion pipelines to enforce granular access controls,
          sanitize sensitive records, and guarantee compliance audit trails before
          data reaches vector databases or AI context windows.
        </p>

        {/* CTAs */}
        <div
          className="mt-9 flex flex-wrap items-center justify-center gap-3"
          style={{ animation: "heroFadeUp 0.7s 0.22s cubic-bezier(0.22,1,0.36,1) both" }}
        >
          <a
            href="https://github.com/cridiv/Auvis#documentation"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 rounded-lg bg-[#0B0C11] px-5 py-2.5 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:bg-[#7B62FC] active:scale-[0.97]"
          >
            Get started
            <ArrowRight className="h-3.5 w-3.5" />
          </a>
          <a
            href="https://github.com/cridiv/Auvis"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-5 py-2.5 text-sm font-medium text-slate-700 shadow-sm transition-all duration-150 hover:border-[#7B62FC]/40 hover:text-[#7B62FC] active:scale-[0.97]"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="currentColor"
              className="h-4 w-4 shrink-0 opacity-80 transition-colors"
            >
              <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.5 11.5 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222 0 1.606-.014 2.898-.014 3.293 0 .322.216.694.825.576C20.565 22.092 24 17.598 24 12.297c0-6.627-5.373-12-12-12" />
            </svg>
            View on GitHub
          </a>
        </div>

        {/* Metrics strip */}
        <div
          className="mt-14 flex flex-wrap items-center justify-center gap-x-6 gap-y-3"
          style={{ animation: "heroFadeUp 0.7s 0.32s cubic-bezier(0.22,1,0.36,1) both" }}
        >
          {[
            { icon: <Gauge className="h-4 w-4" />, label: "19.7k req/s" },
            { icon: <Zap className="h-4 w-4" />, label: "< 1ms overhead" },
            { icon: <ShieldCheck className="h-4 w-4" />, label: "0% error rate (400k reqs)" },
          ].map(({ icon, label }, i) => (
            <React.Fragment key={label}>
              {i > 0 && <span className="hidden text-slate-200 sm:block">|</span>}
              <div className="flex items-center gap-1.5 text-[0.82rem] font-medium text-slate-500">
                <span className="text-[#7B62FC]">{icon}</span>
                {label}
              </div>
            </React.Fragment>
          ))}
        </div>

      </div>

      <style>{`
        @keyframes heroFadeUp {
          from { opacity: 0; transform: translateY(20px); }
          to   { opacity: 1; transform: translateY(0); }
        }
      `}</style>
    </div>
  );
};

export default Hero;

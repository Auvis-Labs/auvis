"use client";

import React from "react";
import Link from "next/link";
import { ArrowRight, BookOpen, Gauge, ShieldCheck, Zap } from "lucide-react";
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
          <Link
            href="/docs/quickstart"
            className="inline-flex items-center gap-2 rounded-lg bg-[#0B0C11] px-5 py-2.5 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:bg-[#7B62FC] active:scale-[0.97]"
          >
            Get started
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
          <Link
            href="/docs"
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-5 py-2.5 text-sm font-medium text-slate-700 shadow-sm transition-all duration-150 hover:border-[#7B62FC]/40 hover:text-[#7B62FC] active:scale-[0.97]"
          >
            <BookOpen className="h-4 w-4 shrink-0 text-slate-500 transition-colors" />
            View docs
          </Link>
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

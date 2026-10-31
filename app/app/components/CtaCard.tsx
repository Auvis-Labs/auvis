"use client";

import React from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { Check, ArrowRight, BookOpen, ShieldCheck } from "lucide-react";

const COMPLIANCE_ITEMS = [
  { name: "CMMC 2.0 Level 2", checked: true },
  { name: "NIST SP 800-171", checked: true },
  { name: "SOC 2 Type II", checked: true },
  { name: "ISO/IEC 27001", checked: true },
  { name: "FedRAMP Moderate", checked: true },
  { name: "HIPAA Safe Harbor", checked: true },
  { name: "GDPR Article 30", checked: true },
  { name: "PCI-DSS v4.0", checked: true },
];

const CtaCard = () => {
  return (
    <section className="dot-plate relative w-full overflow-hidden border-t border-slate-200/80 bg-slate-50/50">
      {/* Subtle Dot Pattern Background */}
      <div
        className="pointer-events-none absolute inset-0 -z-10 opacity-60"
        style={{
          backgroundImage:
            "radial-gradient(circle at 1px 1px, #CBD5E1 1px, transparent 0)",
          backgroundSize: "20px 20px",
        }}
      />

      <div className="mx-auto grid w-full max-w-[1440px] items-center gap-12 px-6 py-24 sm:py-36 lg:grid-cols-[1.1fr_minmax(0,460px)]">
        {/* Left Column: Copy & Actions */}
        <div>
          {/* <div className="inline-flex items-center gap-2 rounded-full border border-purple-200 bg-purple-50 px-3.5 py-1 text-xs font-mono font-semibold tracking-wider text-[#7B62FC]">
            <ShieldCheck className="h-3.5 w-3.5" />
            <span>READY TO GOVERN YOUR AI PIPELINE?</span>
          </div> */}

          <h2 className="mt-5 max-w-2xl text-4xl font-bold tracking-tight text-[#0B0C11] sm:text-5xl lg:text-6xl">
            Stop waiting on the security review.
          </h2>

          <p className="mt-5 max-w-xl text-base text-slate-600 sm:text-lg leading-relaxed">
            <span className="font-semibold text-slate-900">19.7k req/s</span>,
            sub-millisecond proxy overhead, zero application code changes. Auvis
            governs the pipeline you already have.
          </p>

          <div className="mt-10 flex flex-wrap items-center gap-4">
            {/* Primary CTA: Read the Docs */}
            <Link
              href="/docs"
              className="group inline-flex items-center gap-2 rounded-lg bg-[#0B0C11] px-6 py-3.5 text-sm font-medium text-white shadow-sm transition-all hover:bg-[#7B62FC] active:scale-[0.98]"
            >
              {/* <BookOpen className="h-4 w-4" /> */}
              <span>Read the Docs</span>
              <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
            </Link>

            {/* Secondary CTA: View on GitHub */}
            <a
              href="https://github.com/cridiv/Auvis"
              target="_blank"
              rel="noopener noreferrer"
              className="group inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-6 py-3.5 text-sm font-medium text-slate-800 shadow-sm transition-all hover:border-[#7B62FC] hover:bg-[#7B62FC]/10 active:scale-[0.98]"
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="text-slate-800 transition-transform group-hover:scale-105"
              >
                <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.5 11.5 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222 0 1.606-.014 2.898-.014 3.293 0 .322.216.694.825.576C20.565 22.092 24 17.598 24 12.297c0-6.627-5.373-12-12-12" />
              </svg>
              <span>View on GitHub</span>
            </a>
          </div>

          <div className="mt-8 flex items-center gap-4 text-xs font-mono text-slate-400">
            <span className="flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              <span>Open source (Apache 2.0)</span>
            </span>
            <span>•</span>
            <span>Zero SDK Lock-in</span>
            <span>•</span>
            <span>Docker Ready</span>
          </div>
        </div>

        {/* Right Column: Floating Compliance Badges Grid with Gradient Fade */}
        <div className="relative hidden self-stretch lg:block">
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-0 top-1/2 -translate-y-1/2 grid select-none grid-cols-2 content-start gap-3"
            style={{
              maskImage: "linear-gradient(155deg, #000 35%, transparent 88%)",
              WebkitMaskImage:
                "linear-gradient(155deg, #000 35%, transparent 88%)",
            }}
          >
            {COMPLIANCE_ITEMS.map((item, idx) => (
              <motion.div
                key={idx}
                initial={{ opacity: 0, y: 10 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ duration: 0.3, delay: idx * 0.05 }}
                className="flex items-center gap-3 rounded-xl border border-slate-200/90 bg-white p-4 shadow-sm"
              >
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-[5px] bg-[#7B62FC] text-white shadow-sm ring-2 ring-purple-100">
                  <Check className="h-3 w-3 stroke-[3]" />
                </span>
                <span className="truncate text-xs font-semibold tracking-tight text-slate-800">
                  {item.name}
                </span>
              </motion.div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
};

export default CtaCard;

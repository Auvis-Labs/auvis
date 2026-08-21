"use client";

import React from "react";
import { motion } from "framer-motion";

// Official compliance standards supported & enforced by Auvis
const FRAMEWORKS = [
  "GDPR (Art. 30)",
  "HIPAA (Safe Harbor)",
  "PCI-DSS v4.0",
  "SOC 2 Type II",
  "CCPA / CPRA",
  "SOX Compliance",
  "ISO/IEC 27001",
  "NIST SP 800-171",
  "FedRAMP Moderate",
  "CMMC 2.0",
];

const Marquee = () => {
  // Duplicate array to ensure seamless infinite looping
  const items = [...FRAMEWORKS, ...FRAMEWORKS];

  return (
    <div
      className="marquee band relative w-full overflow-hidden border-y border-slate-200/80 bg-zinc-100/90 py-6 backdrop-blur-sm"
      aria-label="Compliance and governance frameworks enforced by Auvis"
    >
      {/* Side Fade Mask Overlays */}
      <div className="pointer-events-none absolute inset-y-0 left-0 z-10 w-24 bg-gradient-to-r from-zinc-100 via-zinc-100/80 to-transparent" />
      <div className="pointer-events-none absolute inset-y-0 right-0 z-10 w-24 bg-gradient-to-l from-zinc-100 via-zinc-100/80 to-transparent" />

      {/* Marquee Track */}
      <div className="marquee-track flex w-max">
        <motion.div
          className="flex shrink-0 items-center gap-8 pr-8"
          animate={{ x: ["0%", "-50%"] }}
          transition={{
            ease: "linear",
            duration: 40,
            repeat: Infinity,
          }}
        >
          {items.map((framework, index) => (
            <div
              key={index}
              className="marquee-item group flex items-center gap-2.5 whitespace-nowrap font-mono text-sm font-medium tracking-tight text-slate-600 transition-colors hover:text-slate-900"
            >
              <span
                aria-hidden="true"
                className="marquee-dot h-1 w-1 rounded-full bg-slate-500 transition-transform group-hover:scale-125"
              />
              <span>{framework}</span>
            </div>
          ))}
        </motion.div>

        {/* Duplicate Track for zero-gap continuous loop */}
        <motion.div
          className="flex shrink-0 items-center gap-8 pr-8"
          aria-hidden="true"
          animate={{ x: ["0%", "-50%"] }}
          transition={{
            ease: "linear",
            duration: 30,
            repeat: Infinity,
          }}
        >
          {items.map((framework, index) => (
            <div
              key={`dup-${index}`}
              className="marquee-item group flex items-center gap-2.5 whitespace-nowrap font-mono text-sm font-medium tracking-tight text-slate-600 transition-colors hover:text-slate-900"
            >
              <span
                aria-hidden="true"
                className="marquee-dot h-0.5 w-0.5 rounded-full bg-slate-500 transition-transform group-hover:scale-125"
              />
              <span>{framework}</span>
            </div>
          ))}
        </motion.div>
      </div>
    </div>
  );
};

export default Marquee;

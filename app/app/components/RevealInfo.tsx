"use client";

import React, { useRef } from "react";
import {
  motion,
  useScroll,
  useTransform,
  MotionValue,
  useMotionTemplate,
} from "framer-motion";

interface WordProps {
  children: string;
  progress: MotionValue<number>;
  range: [number, number];
}

const Word = ({ children, progress, range }: WordProps) => {
  // Explicitly clamp values so once revealed, words stay 100% sharp with 0 blur and 1 opacity
  const opacity = useTransform(progress, range, [0.2, 1], { clamp: true });
  const blur = useTransform(progress, range, [2, 0], { clamp: true });

  return (
    <span className="relative mr-[0.28em] inline-block whitespace-nowrap">
      <motion.span
        style={{
          opacity,
          filter: useMotionTemplate`blur(${blur}px)`,
        }}
        className="inline-block text-white"
      >
        {children}
      </motion.span>
    </span>
  );
};

const RevealInfo = () => {
  const containerRef = useRef<HTMLDivElement>(null);

  const { scrollYProgress } = useScroll({
    target: containerRef,
    offset: ["start start", "end end"],
  });

  const statement =
    "When companies connect internal data to AI models, every source access control disappears. HR records, salary files, and customer PII get flattened into vector databases with zero permissions. Auvis sits directly inside your pipelines to enforce governance in flight masking, redacting, and auditing every record before your AI ever sees it.";

  const words = statement.split(" ");
  const totalWords = words.length;
  // Finish revealing all words by 85% scroll so the entire text stays fully sharp & locked while viewing
  const scrollRangeEnd = 0.85;

  return (
    <section ref={containerRef} className="relative h-[250vh] w-full">
      {/* Sticky Fullscreen Reveal Container */}
      <div className="sticky top-0 flex min-h-screen w-full flex-col justify-end overflow-hidden px-6 pb-20 pt-16 sm:px-12 md:pb-28 lg:px-24">
        {/* Multilayered Violet / Aurora Silk Gradient Background */}
        <div className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
          {/* Base Vertical Gradient */}
          <div
            className="absolute inset-0 h-full w-full"
            style={{
              background:
                "linear-gradient(180deg, #FFFFFF 0%, #F5F3FF 10%, #C4B5FD 28%, #6343E6 48%, #3B1FA8 68%, #1A0D52 86%, #0C0628 100%)",
            }}
          />

          {/* Top Organic Wave Transition */}
          <svg
            className="absolute top-0 left-0 w-full h-[320px] object-cover opacity-90"
            viewBox="0 0 1731 302"
            fill="none"
            preserveAspectRatio="none"
          >
            <path
              d="M 0 0 C 0 0 633.591 335.721 714 299.293 C 794.409 262.864 981.403 177.473 1200.5 94.793 C 1419.597 12.113 1731 0 1731 0 Z"
              fill="url(#wave-gradient)"
            />
            <defs>
              <linearGradient
                id="wave-gradient"
                x1="0.5"
                y1="0"
                x2="0.5"
                y2="1"
              >
                <stop offset="0%" stopColor="#FFFFFF" stopOpacity="1" />
                <stop offset="50%" stopColor="#FFFFFF" stopOpacity="0.75" />
                <stop offset="100%" stopColor="#FFFFFF" stopOpacity="0" />
              </linearGradient>
            </defs>
          </svg>

          {/* Horizontal Silk / Aurora Light Streaks */}
          <div
            className="absolute top-[35%] -left-[10%] w-[120%] h-[180px] opacity-60 blur-3xl"
            style={{
              background:
                "linear-gradient(90deg, rgba(147, 112, 219, 0) 0%, rgba(139, 92, 246, 0.7) 25%, rgba(196, 181, 253, 0.9) 50%, rgba(99, 102, 241, 0.7) 75%, rgba(147, 112, 219, 0) 100%)",
            }}
          />

          <div
            className="absolute top-[52%] -left-[10%] w-[120%] h-[220px] opacity-75 blur-3xl"
            style={{
              background:
                "linear-gradient(90deg, rgba(67, 24, 184, 0) 0%, rgba(91, 33, 182, 0.8) 30%, rgba(124, 58, 237, 0.95) 55%, rgba(79, 70, 229, 0.8) 80%, rgba(67, 24, 184, 0) 100%)",
            }}
          />

          <div
            className="absolute top-[68%] -left-[5%] w-[110%] h-[140px] opacity-50 blur-2xl"
            style={{
              background:
                "linear-gradient(90deg, rgba(30, 10, 90, 0) 0%, rgba(76, 29, 149, 0.7) 40%, rgba(109, 40, 217, 0.8) 60%, rgba(30, 10, 90, 0) 100%)",
            }}
          />

          {/* Subtle Horizontal Mesh Glow */}
          <div className="absolute inset-0 bg-[radial-gradient(ellipse_80%_40%_at_50%_60%,rgba(167,139,250,0.15),transparent)] pointer-events-none" />
        </div>

        {/* Content Container */}
        <div className="mx-auto w-full max-w-5xl">
          {/* Pill Badge: THE SHIFT */}
          <div className="mb-6">
            <span className="inline-block rounded-xl border border-white/50 bg-white/10 px-2.5 py-0.5 text-[11px] font-mono font-medium tracking-wider text-[#black] backdrop-blur-sm shadow-sm">
              THE GAP
            </span>
          </div>

          {/* Progressive Text Reveal Block */}
          <p className="font-display flex flex-wrap text-2xl font-normal leading-[1.3] tracking-tight text-white sm:text-3xl md:text-4xl lg:text-[38px] lg:leading-[1.32]">
            {words.map((word, i) => {
              const start = (i / totalWords) * scrollRangeEnd;
              const step = (1 / totalWords) * scrollRangeEnd;
              const end = Math.min(start + step, 1);

              return (
                <Word key={i} progress={scrollYProgress} range={[start, end]}>
                  {word}
                </Word>
              );
            })}
          </p>
        </div>
      </div>
    </section>
  );
};

export default RevealInfo;

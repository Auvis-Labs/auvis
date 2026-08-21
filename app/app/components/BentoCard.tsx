"use client";

import React, { useState } from "react";
import { motion } from "framer-motion";
import Image from "next/image";
import {
  FileText,
  Check,
  Folder,
  CornerDownRight,
  Bot,
  ShieldCheck,
  Plus,
  ChevronsUpDown,
  Code2,
  Database,
  Layers,
  MessageSquare,
  Sparkles,
  Activity,
  Workflow,
  Cpu,
  Lock,
  FileSpreadsheet,
  Server,
  Zap,
} from "lucide-react";

// Color palette constants for heat map
const HEAT_COLORS: Record<number, string> = {
  0: "#F1F5F9", // Slate-100 (Unmatched / Inactive)
  1: "#DDD6FE", // Violet-200 (Low volume policy hits)
  2: "#A78BFA", // Violet-400 (Active transform)
  3: "#7B62FC", // Signature Auvis Purple (High throughput policy enforcement)
  4: "#4C1D95", // Violet-900 (Fail-closed & Redaction blocks)
};

// Compliance & Policy standards on ingestion
const FRAMEWORKS = [
  { name: "GDPR Minimization", delay: 0 },
  { name: "HIPAA Safe Harbor", delay: 0.6 },
  { name: "SOC 2 Access Control", delay: 1.2 },
  { name: "PCI-DSS Card Masking", delay: 1.8 },
];

// Ingestion Stream 1: RAG Embedding Pipeline
const RAG_STREAM_ROWS = [
  {
    label: "",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "EMAIL.M",
    cells: [0, 2, 0, 0, 3, 0, 0, 2, 0, 0, 4, 0, 2, 0, 0, 3, 0, 0, 2, 0, 2],
  },
  {
    label: "SSN.REM",
    cells: [2, 0, 0, 2, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 3, 0, 0, 2, 0],
  },
  {
    label: "PHONE.M",
    cells: [0, 0, 2, 0, 0, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0],
  },
  {
    label: "JWT.ROLE",
    cells: [3, 0, 0, 4, 0, 2, 0, 0, 3, 0, 0, 2, 0, 3, 0, 0, 4, 0, 0, 3, 2],
  },
  {
    label: "CARD.RED",
    cells: [0, 2, 0, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 3],
  },
  {
    label: "PHI.SCAN",
    cells: [2, 0, 3, 0, 2, 0, 0, 2, 0, 3, 0, 0, 2, 0, 2, 0, 2, 0, 0, 0, 0],
  },
  {
    label: "FAIL.CLS",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "ACL.TAG",
    cells: [0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 2, 0, 0, 0],
  },
  {
    label: "",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
];

// Ingestion Stream 2: Kafka Production Stream
const KAFKA_STREAM_ROWS = [
  {
    label: "",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "SALARY.M",
    cells: [2, 0, 0, 2, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 3, 0, 0, 2, 0],
  },
  {
    label: "NAME.TOK",
    cells: [0, 2, 0, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 3],
  },
  {
    label: "M&A.DROP",
    cells: [2, 0, 3, 0, 2, 0, 0, 2, 0, 3, 0, 0, 2, 0, 2, 0, 2, 0, 0, 0, 0],
  },
  {
    label: "DLQ.ROUT",
    cells: [0, 0, 2, 0, 0, 2, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0],
  },
  {
    label: "AVRO.DEC",
    cells: [0, 0, 2, 0, 0, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0],
  },
  {
    label: "JSONL.WR",
    cells: [0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0],
  },
  {
    label: "LUHN.CHK",
    cells: [2, 0, 0, 0, 2, 0, 0, 2, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "FSYNC.OK",
    cells: [0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 2, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
];

// Ingestion Stream 3: Confluence & Postgres Sync
const CONFLUENCE_STREAM_ROWS = [
  {
    label: "",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "DOC.PARS",
    cells: [0, 2, 0, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 3],
  },
  {
    label: "PII.HYBR",
    cells: [0, 0, 2, 0, 0, 0, 0, 3, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0],
  },
  {
    label: "LLM.INTC",
    cells: [0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0],
  },
  {
    label: "PERM.XFER",
    cells: [2, 0, 3, 0, 2, 0, 0, 2, 0, 3, 0, 0, 2, 0, 2, 0, 2, 0, 0, 0, 0],
  },
  {
    label: "ROW.COND",
    cells: [0, 2, 0, 0, 3, 0, 0, 2, 0, 0, 0, 0, 2, 0, 2, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "SINK.VEC",
    cells: [0, 0, 2, 0, 0, 2, 0, 0, 0, 2, 0, 0, 2, 0, 0, 0, 0, 0, 2, 0, 0],
  },
  {
    label: "LOG.DISP",
    cells: [2, 0, 0, 0, 2, 0, 0, 2, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0],
  },
  {
    label: "",
    cells: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
  },
];

// Target AI Infrastructure destinations
const TARGET_DESTINATIONS = [
  {
    title: "Vector Databases",
    subtitle: "pgvector · Pinecone · Weaviate",
    icon: Database,
    badge: "Clean embeddings",
    pulseDelay: 0,
  },
  {
    title: "RAG & LLM Apps",
    subtitle: "Claude · Cursor · LangChain",
    icon: Bot,
    badge: "Zero leak context",
    pulseDelay: 0.8,
  },
  {
    title: "Audit & Compliance",
    subtitle: "JSONL · Splunk · Datadog",
    icon: ShieldCheck,
    badge: "Full field lineage",
    pulseDelay: 1.6,
  },
];

// Reusable Heatmap Grid Component
const HeatmapGrid = ({
  rows,
  startDate = "12:00:00",
  endDate = "12:00:05",
}: {
  rows: { label: string; cells: number[] }[];
  startDate?: string;
  endDate?: string;
}) => {
  return (
    <div
      className="mt-2 grid gap-[1.5px]"
      style={{ gridTemplateColumns: "36px repeat(21, minmax(0, 1fr))" }}
    >
      {rows.map((row, rIdx) => (
        <React.Fragment key={rIdx}>
          <span className="flex items-center justify-end border-r border-slate-200 pr-1 font-mono text-[6.5px] leading-none text-slate-400">
            {row.label}
          </span>
          {row.cells.map((cellVal, cIdx) => (
            <motion.span
              key={cIdx}
              initial={{ scale: 0.8, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              transition={{
                duration: 0.2,
                delay: (rIdx * 21 + cIdx) * 0.0015,
                ease: "easeOut",
              }}
              whileHover={{ scale: 1.35, zIndex: 10 }}
              style={{
                backgroundColor: HEAT_COLORS[cellVal] || HEAT_COLORS[0],
              }}
              className="aspect-square min-w-0 rounded-[1.5px] transition-colors"
            />
          ))}
        </React.Fragment>
      ))}

      {/* Timestamp Range Footer */}
      <span
        className="mt-0.5 flex items-baseline justify-between border-t border-slate-200 pt-0.5"
        style={{ gridColumn: "2 / -1" }}
      >
        <span className="font-mono text-[6.5px] text-slate-400">
          {startDate}
        </span>
        <span className="font-mono text-[6.5px] text-slate-400">{endDate}</span>
      </span>
    </div>
  );
};

const BentoCard = () => {
  const [activePipeline, setActivePipeline] =
    useState<string>("rag-embeddings-v1");

  return (
    <section className="hidden md:block relative w-full overflow-hidden bg-slate-50/50 py-20">
      {/* Background Ambient Glow */}
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-purple-100/40 via-transparent to-transparent" />

      <div className="mx-auto w-full max-w-[1440px] px-4 sm:px-6 lg:px-8">
        {/* Section Header */}
        <div className="mb-12 text-center">
          {/* <div className="inline-flex items-center gap-2 rounded-full border border-purple-200 bg-purple-50/80 px-3 py-1 text-xs font-semibold text-[#7B62FC]">
            <Zap className="h-3.5 w-3.5" />
            <span>IN-FLIGHT POLICY ENFORCEMENT & DATA GOVERNANCE</span>
          </div> */}
          <h2 className="mt-3 text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl">
            IN-FLIGHT POLICY ENFORCEMENT & DATA GOVERNANCE
          </h2>
          <p className="mx-auto mt-3 max-w-2xl text-base text-slate-600">
            Auvis intercepts ingestion streams in flight, masking PII,
            evaluating row-level policies in &lt;5ms, and propagating source
            permissions directly to vector databases.
          </p>
        </div>

        {/* Main Bento Stage */}
        <div className="w-full">
          <div className="flex flex-col items-center justify-between gap-0 lg:flex-row lg:items-center">
            {/* 1. LEFT COLUMN: Policies & Standards */}
            <div className="w-full flex-none lg:w-[210px]">
              <div className="flex flex-col items-center">
                <div className="flex w-full items-center">
                  <div className="min-w-0 flex-1">
                    <motion.div
                      whileHover={{ y: -2 }}
                      className="overflow-hidden rounded-[10px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/50"
                    >
                      {/* Card Header */}
                      <div className="flex items-center gap-1.5 border-b border-slate-100 bg-slate-50/70 px-3 py-2">
                        <span className="h-1.5 w-1.5 rounded-full bg-slate-300"></span>
                        <span className="h-1.5 w-1.5 rounded-full bg-slate-300"></span>
                        <span className="h-1.5 w-1.5 rounded-full bg-slate-300"></span>
                        <span className="ml-1.5 font-mono text-[9px] tracking-tight text-slate-500">
                          Policy · Enforcement
                        </span>
                      </div>

                      {/* Frameworks List */}
                      <ul className="divide-y divide-slate-100/80 px-3 py-1">
                        {FRAMEWORKS.map((fw, idx) => (
                          <li
                            key={idx}
                            className="flex items-center gap-2 py-[7px]"
                          >
                            <FileText className="h-3 w-3 shrink-0 text-slate-400" />
                            <span className="font-mono text-[9.5px] font-medium tracking-tight text-slate-800 truncate">
                              {fw.name}
                            </span>
                            <motion.span
                              animate={{
                                scale: [1, 1.18, 1],
                                opacity: [0.85, 1, 0.85],
                              }}
                              transition={{
                                duration: 2.2,
                                repeat: Infinity,
                                delay: fw.delay,
                                ease: "easeInOut",
                              }}
                              className="ml-auto flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-[3px] bg-purple-50 text-[#7B62FC] ring-1 ring-purple-200/80"
                            >
                              <Check className="h-2 w-2 stroke-[3.5]" />
                            </motion.span>
                          </li>
                        ))}
                      </ul>
                    </motion.div>
                  </div>

                  {/* Horizontal Flow Line to Center (Desktop) */}
                  <div className="hidden flex-none items-center lg:flex">
                    <svg
                      aria-hidden="true"
                      className="w-8 flex-none overflow-visible"
                      viewBox="0 0 32 8"
                      fill="none"
                    >
                      <motion.path
                        d="M0 4 H32"
                        stroke="#7B62FC"
                        strokeWidth="1.5"
                        strokeDasharray="4 4"
                        animate={{ strokeDashoffset: [0, -16] }}
                        transition={{
                          duration: 1.2,
                          repeat: Infinity,
                          ease: "linear",
                        }}
                      />
                    </svg>
                  </div>
                </div>

                <p className="mt-3 text-center text-[10px] font-medium text-slate-400 lg:pr-8">
                  Evaluated in-flight (&lt;5ms)
                </p>
              </div>
            </div>

            {/* 2. CENTER COLUMN: Auvis Core Ingestion & Transform Gateway */}
            <div className="min-w-0 flex-1 w-full max-w-[940px]">
              <div className="relative mx-auto w-full text-left">
                {/* Floating Badges */}
                <motion.span
                  animate={{ y: [0, -4, 0] }}
                  transition={{
                    duration: 4,
                    repeat: Infinity,
                    ease: "easeInOut",
                  }}
                  className="absolute right-[4%] -top-3.5 z-20 hidden items-center gap-1.5 whitespace-nowrap rounded-full border border-slate-200/80 bg-white/95 px-2.5 py-1 text-[10px] font-medium text-slate-600 shadow-md shadow-slate-200/70 backdrop-blur-md sm:flex"
                >
                  <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#7B62FC]"></span>
                  In-flight masking & regex check digit verification
                </motion.span>

                <motion.span
                  animate={{ y: [0, 4, 0] }}
                  transition={{
                    duration: 4.5,
                    repeat: Infinity,
                    ease: "easeInOut",
                    delay: 1,
                  }}
                  className="absolute -bottom-3 right-[6%] z-20 hidden items-center gap-1.5 whitespace-nowrap rounded-full border border-slate-200/80 bg-white/95 px-2.5 py-1 text-[10px] font-medium text-slate-600 shadow-md shadow-slate-200/70 backdrop-blur-md sm:flex"
                >
                  <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500"></span>
                  Fail-closed synchronous writes to pgvector & Pinecone
                </motion.span>

                <motion.span
                  animate={{ y: [0, -4, 0] }}
                  transition={{
                    duration: 5,
                    repeat: Infinity,
                    ease: "easeInOut",
                    delay: 0.5,
                  }}
                  className="absolute -left-3 bottom-[28%] z-20 hidden items-center gap-1.5 whitespace-nowrap rounded-full border border-slate-200/80 bg-white/95 px-2.5 py-1 text-[10px] font-medium text-slate-700 shadow-md shadow-slate-200/70 backdrop-blur-md sm:flex"
                >
                  <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-slate-900"></span>
                  4-Bucket JSONL audit trail · Splunk & Datadog ready
                </motion.span>

                {/* Main Gateway Frame */}
                <div className="overflow-hidden rounded-xl border border-slate-200/90 bg-white shadow-2xl shadow-slate-200/80">
                  {/* Window Bar */}
                  <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50/90 px-3.5 py-2.5">
                    <div className="flex items-center gap-1.5">
                      <span className="h-2.5 w-2.5 rounded-full bg-slate-900"></span>
                      <span className="h-2.5 w-2.5 rounded-full bg-slate-500"></span>
                      <span className="h-2.5 w-2.5 rounded-full bg-slate-300"></span>
                    </div>
                    <div className="flex items-center gap-1.5 font-mono text-[9.5px] text-slate-500">
                      <Activity className="h-3 w-3 text-[#7B62FC]" />
                      <span>
                        auvis.gateway:8080 · 19.7k req/s · 2.5ms avg · &lt;1ms overhead
                      </span>
                    </div>
                  </div>

                  {/* Window Content */}
                  <div className="flex min-h-[460px] items-stretch bg-slate-50/40">
                    {/* Left Icon Rail */}
                    <div className="relative flex w-11 shrink-0 flex-col items-center border-r border-slate-100 bg-white py-3">
                      <div className="flex h-6 w-6 items-center justify-center rounded-md bg-[#7B62FC]/10 text-[#7B62FC]">
                        <Workflow className="h-3.5 w-3.5" />
                      </div>

                      <div className="relative mt-3">
                        <span className="absolute -left-[14px] top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full bg-[#7B62FC]"></span>
                        <button className="flex h-7 w-7 items-center justify-center rounded-[8px] bg-slate-900 text-white shadow-sm ring-2 ring-purple-100 transition-transform active:scale-95">
                          <Layers className="h-3.5 w-3.5" />
                        </button>
                      </div>

                      <div className="mt-auto flex flex-col items-center gap-2 text-slate-400">
                        <div className="flex h-6 w-6 items-center justify-center rounded-full bg-purple-100 text-[9px] font-semibold text-[#7B62FC]">
                          <Image
                            src="/auvis-final.svg"
                            alt="Auvis"
                            width={14}
                            height={14}
                            className="h-[14px] w-[14px]"
                          />
                        </div>
                        <Lock className="h-3 w-3 text-slate-400 hover:text-slate-600" />
                        {/* <Sparkles className="h-3 w-3 text-slate-400 hover:text-slate-600" /> */}
                      </div>
                    </div>

                    {/* Sub-Sidebar: Ingestion Pipeline Navigator */}
                    <div className="hidden w-[135px] shrink-0 flex-col border-r border-slate-200/70 bg-white px-2 py-3 sm:flex">
                      {/* Environment Selector */}
                      <button className="flex w-full items-center justify-between gap-1 rounded-md px-1.5 py-1 text-left hover:bg-slate-50">
                        <span className="flex items-center gap-1 truncate text-[10.5px] font-medium text-slate-800">
                          Gateway v0.2.6
                        </span>
                        <ChevronsUpDown className="h-2.5 w-2.5 shrink-0 text-slate-400" />
                      </button>

                      {/* Pipelines List */}
                      <div className="mt-2.5 flex flex-col gap-0.5">
                        <div className="flex items-center gap-1.5 rounded-[6px] bg-purple-50 px-2 py-1 text-[10px] font-medium text-[#7B62FC]">
                          <Folder className="h-3 w-3 shrink-0 text-[#7B62FC]" />
                          <span className="min-w-0 flex-1 truncate">
                            Pipelines
                          </span>
                          <span className="font-mono text-[8.5px] text-[#7B62FC]/80">
                            3
                          </span>
                        </div>

                        <button
                          onClick={() => setActivePipeline("rag-embeddings-v1")}
                          className={`flex items-center gap-1 rounded-[6px] px-1.5 py-1 text-[9.5px] transition-colors ${activePipeline === "rag-embeddings-v1"
                              ? "bg-slate-100 font-medium text-slate-900"
                              : "text-slate-500 hover:bg-slate-50 hover:text-slate-800"
                            }`}
                        >
                          <CornerDownRight className="h-2.5 w-2.5 shrink-0 text-slate-400" />
                          <span className="min-w-0 flex-1 truncate">
                            rag-embeddings
                          </span>
                        </button>

                        <button
                          onClick={() => setActivePipeline("kafka-ingest-prod")}
                          className={`flex items-center gap-1 rounded-[6px] px-1.5 py-1 text-[9.5px] transition-colors ${activePipeline === "kafka-ingest-prod"
                              ? "bg-slate-100 font-medium text-slate-900"
                              : "text-slate-500 hover:bg-slate-50 hover:text-slate-800"
                            }`}
                        >
                          <CornerDownRight className="h-2.5 w-2.5 shrink-0 text-slate-400" />
                          <span className="min-w-0 flex-1 truncate">
                            kafka-stream
                          </span>
                        </button>

                        <button
                          onClick={() => setActivePipeline("confluence-sync")}
                          className={`flex items-center gap-1 rounded-[6px] px-1.5 py-1 text-[9.5px] transition-colors ${activePipeline === "confluence-sync"
                              ? "bg-slate-100 font-medium text-slate-900"
                              : "text-slate-500 hover:bg-slate-50 hover:text-slate-800"
                            }`}
                        >
                          <CornerDownRight className="h-2.5 w-2.5 shrink-0 text-slate-400" />
                          <span className="min-w-0 flex-1 truncate">
                            confluence-sync
                          </span>
                        </button>
                      </div>

                      {/* Engine Modules Group */}
                      <span className="mb-1 mt-3 px-2 text-[8px] font-semibold uppercase tracking-wider text-slate-400">
                        Governance Engine
                      </span>
                      <div className="flex flex-col gap-0.5">
                        <div className="flex items-center gap-1.5 rounded-[6px] px-2 py-1 text-[10px] text-slate-600 hover:bg-slate-50">
                          <Bot className="h-3 w-3 shrink-0 text-slate-400" />
                          <span className="min-w-0 flex-1 truncate">
                            Hybrid PII Scan
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 rounded-[6px] px-2 py-1 text-[10px] text-slate-600 hover:bg-slate-50">
                          <Layers className="h-3 w-3 shrink-0 text-slate-400" />
                          <span className="min-w-0 flex-1 truncate">
                            Transform Rules
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 rounded-[6px] px-2 py-1 text-[10px] text-slate-600 hover:bg-slate-50">
                          <FileSpreadsheet className="h-3 w-3 shrink-0 text-slate-400" />
                          <span className="min-w-0 flex-1 truncate">
                            JSONL Audit Log
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Main Workspace Stage Cards */}
                    <div className="min-w-0 flex-1 p-3.5">
                      <div className="flex items-center justify-between">
                        <p className="font-mono text-[8.5px] font-semibold uppercase tracking-[0.14em] text-slate-400">
                          Active Ingestion Pipelines & Policy Telemetry
                        </p>
                        <span className="flex items-center gap-1 text-[9px] text-[#7B62FC]">
                          <Cpu className="h-2.5 w-2.5" />
                          <span>Hot-reload active</span>
                        </span>
                      </div>

                      {/* 3 Pipeline Stream Cards */}
                      <div className="mt-2.5 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                        {/* Card 1: New Pipeline Connector Tile */}
                        <motion.div
                          whileHover={{ scale: 1.02 }}
                          className="flex aspect-square cursor-pointer flex-col items-center justify-center gap-1.5 rounded-[10px] border border-dashed border-slate-300/80 bg-white/70 text-slate-500 transition-all hover:border-[#7B62FC]/50 hover:bg-purple-50/30 hover:text-[#7B62FC]"
                          style={{
                            backgroundImage:
                              "radial-gradient(circle at 1px 1px, #E2E8F0 1px, transparent 0)",
                            backgroundSize: "9px 9px",
                          }}
                        >
                          <div className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-100 text-slate-600 transition-colors group-hover:bg-purple-100 group-hover:text-[#7B62FC]">
                            <Plus className="h-3.5 w-3.5" />
                          </div>
                          <span className="text-[10px] font-medium">
                            New Source Pipeline
                          </span>
                          <span className="text-[8px] text-slate-400">
                            Kafka · Postgres · S3
                          </span>
                        </motion.div>

                        {/* Card 2: rag-embeddings-v1 */}
                        <motion.div
                          whileHover={{ y: -2 }}
                          className={`flex aspect-square min-w-0 flex-col rounded-[10px] border bg-white p-3 shadow-sm transition-all ${activePipeline === "rag-embeddings-v1"
                              ? "border-[#7B62FC]/60 ring-2 ring-purple-100"
                              : "border-slate-200"
                            }`}
                        >
                          <div className="flex items-start justify-between gap-1">
                            <div>
                              <span className="block text-[12px] font-semibold text-slate-900">
                                rag-embeddings-v1
                              </span>
                              <span className="block text-[9.5px] text-slate-400">
                                <span className="font-mono font-medium text-slate-600">
                                  19.7k
                                </span>{" "}
                                req/s · &lt;1ms
                              </span>
                            </div>
                            <span className="font-mono text-[8px] text-slate-400">
                              ‹ 1/3 ›
                            </span>
                          </div>

                          {/* Activity Heatmap */}
                          <HeatmapGrid
                            rows={RAG_STREAM_ROWS}
                            startDate="12:00:00"
                            endDate="12:00:05"
                          />

                          {/* Tags & Frameworks */}
                          <div className="mt-auto flex flex-wrap items-center gap-1 pt-1.5">
                            <span className="inline-flex items-baseline gap-1 rounded-full bg-purple-50 px-2 py-0.5 text-[8.5px] font-medium text-purple-900">
                              GDPR Minimization
                              <span className="font-mono font-semibold text-[#7B62FC]">
                                100%
                              </span>
                            </span>
                          </div>

                          {/* Metric Footer */}
                          <div className="mt-2 flex items-center justify-between border-t border-slate-100 pt-1.5 text-[9px] text-slate-500">
                            <span>
                              PII Masked{" "}
                              <span className="font-mono font-semibold text-slate-900">
                                99.8%
                              </span>
                            </span>
                            <span>
                              Sink:{" "}
                              <span className="font-mono font-semibold text-[#7B62FC]">
                                pgvector
                              </span>
                            </span>
                          </div>
                        </motion.div>

                        {/* Card 3: kafka-ingest-prod */}
                        <motion.div
                          whileHover={{ y: -2 }}
                          className={`flex aspect-square min-w-0 flex-col rounded-[10px] border bg-white p-3 shadow-sm transition-all ${activePipeline === "kafka-ingest-prod"
                              ? "border-[#7B62FC]/60 ring-2 ring-purple-100"
                              : "border-slate-200"
                            }`}
                        >
                          <div className="flex items-start justify-between gap-1">
                            <div>
                              <span className="block text-[12px] font-semibold text-slate-900">
                                kafka-ingest-prod
                              </span>
                              <span className="block text-[9.5px] text-slate-400">
                                <span className="font-mono font-medium text-slate-600">
                                  480k
                                </span>{" "}
                                msg/min · 0% err
                              </span>
                            </div>
                            <span className="font-mono text-[8px] text-slate-400">
                              ‹ 2/3 ›
                            </span>
                          </div>

                          {/* Activity Heatmap */}
                          <HeatmapGrid
                            rows={KAFKA_STREAM_ROWS}
                            startDate="12:00:00"
                            endDate="12:00:05"
                          />

                          {/* Tags */}
                          <div className="mt-auto flex flex-wrap items-center gap-1 pt-1.5">
                            <span className="inline-flex items-baseline gap-1 rounded-full bg-purple-50 px-1.5 py-0.5 text-[8px] font-medium text-purple-900">
                              HIPAA PHI
                              <span className="font-mono font-semibold text-[#7B62FC]">
                                Pass
                              </span>
                            </span>
                            <span className="inline-flex items-baseline gap-1 rounded-full bg-slate-100 px-1.5 py-0.5 text-[8px] font-medium text-slate-700">
                              Schema Avro
                              <span className="font-mono font-semibold text-slate-900">
                                Sync
                              </span>
                            </span>
                          </div>

                          {/* Metric Footer */}
                          <div className="mt-2 flex items-center justify-between border-t border-slate-100 pt-1.5 text-[9px] text-slate-500">
                            <span>
                              Durability{" "}
                              <span className="font-mono font-semibold text-slate-900">
                                100%
                              </span>
                            </span>
                            <span>
                              Sink:{" "}
                              <span className="font-mono font-semibold text-[#7B62FC]">
                                Weaviate
                              </span>
                            </span>
                          </div>
                        </motion.div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* 3. RIGHT COLUMN: Target AI Destinations */}
            <div className="w-full flex-none lg:w-[220px]">
              <div className="flex flex-col items-center">
                <div className="flex w-full items-center">
                  {/* Branching SVG Connector (Desktop) */}
                  <div
                    aria-hidden="true"
                    className="relative hidden h-[220px] w-12 flex-none self-stretch lg:block"
                  >
                    <svg
                      className="absolute inset-0 h-full w-full overflow-visible"
                      viewBox="0 0 56 180"
                      preserveAspectRatio="none"
                      fill="none"
                    >
                      <motion.path
                        d="M0 90 C24 90 24 26 56 26"
                        stroke="#7B62FC"
                        strokeWidth="1.5"
                        strokeDasharray="4 4"
                        vectorEffect="non-scaling-stroke"
                        animate={{ strokeDashoffset: [0, -16] }}
                        transition={{
                          duration: 1.4,
                          repeat: Infinity,
                          ease: "linear",
                        }}
                      />
                      <motion.path
                        d="M0 90 H56"
                        stroke="#7B62FC"
                        strokeWidth="1.5"
                        strokeDasharray="4 4"
                        vectorEffect="non-scaling-stroke"
                        animate={{ strokeDashoffset: [0, -16] }}
                        transition={{
                          duration: 1.4,
                          repeat: Infinity,
                          ease: "linear",
                        }}
                      />
                      <motion.path
                        d="M0 90 C24 90 24 154 56 154"
                        stroke="#7B62FC"
                        strokeWidth="1.5"
                        strokeDasharray="4 4"
                        vectorEffect="non-scaling-stroke"
                        animate={{ strokeDashoffset: [0, -16] }}
                        transition={{
                          duration: 1.4,
                          repeat: Infinity,
                          ease: "linear",
                        }}
                      />
                    </svg>
                  </div>

                  {/* Destinations Cards List */}
                  <div className="flex min-w-0 flex-1 flex-col gap-2.5">
                    {TARGET_DESTINATIONS.map((dest, idx) => {
                      const IconComponent = dest.icon;
                      return (
                        <motion.div
                          key={idx}
                          whileHover={{ x: 3, scale: 1.01 }}
                          className="flex flex-col gap-2 rounded-[10px] border border-slate-200/90 bg-white p-3 shadow-md shadow-slate-200/40"
                        >
                          <div className="flex items-start justify-between">
                            <div className="flex h-6 w-6 items-center justify-center rounded-md bg-purple-50 text-[#7B62FC]">
                              <IconComponent className="h-3.5 w-3.5" />
                            </div>
                            <motion.span
                              animate={{
                                scale: [1, 1.2, 1],
                                opacity: [0.7, 1, 0.7],
                              }}
                              transition={{
                                duration: 2,
                                repeat: Infinity,
                                delay: dest.pulseDelay,
                                ease: "easeInOut",
                              }}
                              className="flex h-4 w-4 items-center justify-center rounded-full bg-emerald-50 text-emerald-600 ring-1 ring-emerald-200"
                            >
                              <Check className="h-2.5 w-2.5 stroke-[3]" />
                            </motion.span>
                          </div>
                          <div className="min-w-0">
                            <span className="block text-[12px] font-semibold text-slate-900">
                              {dest.title}
                            </span>
                            <span className="mt-0.5 block truncate text-[9.5px] text-slate-400">
                              {dest.subtitle}
                            </span>
                          </div>
                        </motion.div>
                      );
                    })}
                  </div>
                </div>

                <p className="mt-3 text-center text-[10px] font-medium text-slate-400 lg:pl-8">
                  Delivered safely to AI sinks
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default BentoCard;

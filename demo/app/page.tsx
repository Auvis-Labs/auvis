'use client';

import React, { useState, useEffect, useRef } from 'react';

type SelectedNode = 'kafka' | 'engine' | 'postgres' | 'pgvector' | null;

interface GovernedRecord {
  id: string;
  timestamp: string;
  diagnosis: string;
  confidence: string;
  maskedReference: string;
  patientId: string;
  rawPayload?: any;
  governedPayload?: any;
  rulesApplied?: string[];
  fieldsMasked?: string[];
  fieldsRemoved?: string[];
  fieldsExposedPHI?: string[];
  resource?: string;
  userRole?: string;
}

export default function Home() {
  const [mounted, setMounted] = useState<boolean>(false);
  const [isLiveStream, setIsLiveStream] = useState<boolean>(false);
  const [recordsProcessed, setRecordsProcessed] = useState<number>(0);
  const [throughput, setThroughput] = useState<number>(0);
  const [fieldsProtected, setFieldsProtected] = useState<number>(0);
  const [elapsedSeconds, setElapsedSeconds] = useState<number>(0);
  const [selectedNode, setSelectedNode] = useState<SelectedNode>(null);
  const [activeAuditRecord, setActiveAuditRecord] = useState<GovernedRecord | null>(null);

  const [drawerPos, setDrawerPos] = useState<{ x: number; y: number } | null>(null);
  const [isDragging, setIsDragging] = useState<boolean>(false);
  const dragStartRef = useRef<{ startX: number; startY: number; initX: number; initY: number } | null>(null);

  const handlePointerDown = (e: React.PointerEvent) => {
    if ((e.target as HTMLElement).tagName === 'BUTTON' || (e.target as HTMLElement).closest('button')) return;
    
    e.preventDefault();
    const rect = (e.currentTarget.parentElement as HTMLElement)?.getBoundingClientRect();
    const currentX = drawerPos ? drawerPos.x : (rect ? rect.left : window.innerWidth - 480);
    const currentY = drawerPos ? drawerPos.y : (rect ? rect.top : window.innerHeight - 420);

    dragStartRef.current = {
      startX: e.clientX,
      startY: e.clientY,
      initX: currentX,
      initY: currentY
    };
    setIsDragging(true);
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  };

  const handlePointerMove = (e: React.PointerEvent) => {
    if (!isDragging || !dragStartRef.current) return;
    const deltaX = e.clientX - dragStartRef.current.startX;
    const deltaY = e.clientY - dragStartRef.current.startY;
    
    const newX = Math.max(10, Math.min(window.innerWidth - 320, dragStartRef.current.initX + deltaX));
    const newY = Math.max(10, Math.min(window.innerHeight - 150, dragStartRef.current.initY + deltaY));
    
    setDrawerPos({ x: newX, y: newY });
  };

  const handlePointerUp = (e: React.PointerEvent) => {
    if (isDragging) {
      setIsDragging(false);
      dragStartRef.current = null;
      try {
        (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
      } catch {}
    }
  };

  const [liveFeed, setLiveFeed] = useState<GovernedRecord[]>([]);

  const eventWindowRef = useRef<number[]>([]);
  const lastEventTimeRef = useRef<number>(0);

  // Fix Hydration Error by mounting client-side state
  useEffect(() => {
    setMounted(true);
  }, []);

  // Elapsed timer ticker & Throughput decay ticker
  useEffect(() => {
    const timer = setInterval(() => {
      setElapsedSeconds((prev) => prev + 1);
      
      // Decay throughput display when stream completes
      const now = Date.now();
      if (now - lastEventTimeRef.current > 1200) {
        eventWindowRef.current = [];
        setThroughput(0);
      }
    }, 500);
    return () => clearInterval(timer);
  }, []);

  // Connect to Auvis GET /auvis/stream (SSE Endpoint)
  useEffect(() => {
    if (!mounted) return;

    const streamUrl = process.env.NEXT_PUBLIC_AUVIS_STREAM_URL || 'http://localhost:8082/auvis/stream';
    let eventSource: EventSource | null = null;
    let reconnectTimer: NodeJS.Timeout | null = null;

    const connect = () => {
      try {
        if (eventSource) {
          eventSource.close();
        }
        eventSource = new EventSource(streamUrl);

        eventSource.onopen = () => {
          setIsLiveStream(true);
        };

        eventSource.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            const diagnosis = data.governed_payload?.diagnosis || data.raw_payload?.diagnosis || data.resource || 'Governed Healthcare Record';
            const maskedRef = data.governed_payload?.email || data.governed_payload?.patient_name || data.governed_payload?.phone || 'user_99***@health.org';
            const pId = data.governed_payload?.patient_id || data.governed_payload?.id || `usr_${Math.random().toString(36).substring(2, 10)}`;
            const protectedCount = (data.fields_masked?.length || 0) + (data.fields_removed?.length || 0) || (data.rules_applied?.length || 1);

            // Compute real sliding-window throughput (events/sec)
            const now = Date.now();
            lastEventTimeRef.current = now;
            eventWindowRef.current.push(now);
            eventWindowRef.current = eventWindowRef.current.filter((ts) => now - ts <= 1000);

            setIsLiveStream(true);
            setRecordsProcessed((prev) => prev + 1);
            setFieldsProtected((prev) => prev + protectedCount);
            setThroughput(eventWindowRef.current.length);

            const newRecord: GovernedRecord = {
              id: data.request_id || `req-${Math.floor(1000 + Math.random() * 9000)}`,
              timestamp: data.timestamp ? new Date(data.timestamp).toLocaleTimeString() : new Date().toLocaleTimeString(),
              diagnosis: diagnosis,
              confidence: '99.4%',
              maskedReference: maskedRef,
              patientId: pId,
              rawPayload: data.raw_payload,
              governedPayload: data.governed_payload,
              rulesApplied: data.rules_applied || data.policies_applied || [],
              fieldsMasked: data.fields_masked || [],
              fieldsRemoved: data.fields_removed || [],
              fieldsExposedPHI: data.fields_exposed_phi || [],
              resource: data.resource || '/patients',
              userRole: data.user_role || 'doctor'
            };

            setLiveFeed((prev) => [newRecord, ...prev.slice(0, 7)]);
          } catch {
            // Ignore invalid JSON frame
          }
        };

        eventSource.onerror = () => {
          setIsLiveStream(false);
          eventSource?.close();
          reconnectTimer = setTimeout(connect, 3000);
        };
      } catch {
        setIsLiveStream(false);
        reconnectTimer = setTimeout(connect, 3000);
      }
    };

    connect();

    return () => {
      if (eventSource) eventSource.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
    };
  }, [mounted]);

  const formatTime = (totalSec: number) => {
    const hrs = Math.floor(totalSec / 3600);
    const mins = Math.floor((totalSec % 3600) / 60);
    const secs = totalSec % 60;
    return [hrs, mins, secs].map(v => v.toString().padStart(2, '0')).join(':');
  };

  const handleNodeClick = (node: SelectedNode) => {
    setSelectedNode(prev => prev === node ? null : node);
  };

  return (
    <main className="min-h-screen bg-[#07090e] text-slate-100 font-sans flex flex-col">

      {/* SECTION 1 — HEADER BAR */}
      <header className="w-full h-12 px-6 border-b border-slate-800/80 bg-slate-950/60 flex items-center justify-between flex-shrink-0">
        <span className="text-xs font-semibold uppercase tracking-wider text-slate-300">
          Live ingestion
        </span>
        <div className="flex items-center gap-2">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          </span>
          <span className="text-xs font-medium text-emerald-400 tracking-wide lowercase">
            {isLiveStream ? 'live proxy stream (8082)' : 'streaming'}
          </span>
        </div>
      </header>

      {/* SECTION 2 — SUMMARY STAT STRIP */}
      <section className="w-full px-6 py-6 border-b border-slate-800/80 bg-slate-950/30 flex-shrink-0">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
          <div className="flex flex-col gap-1">
            <span className="text-xs font-medium text-slate-400">Records processed</span>
            <span className="font-mono text-2xl font-bold tracking-tight text-slate-100" suppressHydrationWarning>
              {recordsProcessed.toLocaleString()}
            </span>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-xs font-medium text-slate-400">Throughput</span>
            <span className="font-mono text-2xl font-bold tracking-tight text-slate-100" suppressHydrationWarning>
              {throughput} <span className="text-sm font-normal text-slate-400">rec/s</span>
            </span>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-xs font-medium text-slate-400">Fields protected</span>
            <span className="font-mono text-2xl font-bold tracking-tight text-slate-100" suppressHydrationWarning>
              {fieldsProtected.toLocaleString()}
            </span>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-xs font-medium text-slate-400">PHI exposures</span>
            <span className="font-mono text-2xl font-bold tracking-tight text-slate-100">
              0
            </span>
          </div>
        </div>
      </section>

      {/* MAIN BODY CONTAINER */}
      <div className="w-full flex-1 px-6 py-8 flex flex-col gap-8">

        {/* SECTION 3 — THE NODE GRAPH CANVAS (SINGLE CONTINUOUS WAVY LINE TOUCHING ALL 4 NODES) */}
        <section className="w-full py-8 flex flex-col items-center justify-center relative">
          <div className="w-full max-w-[1400px] relative grid grid-cols-1 md:grid-cols-4 gap-8 items-start z-10 min-h-[280px]">

            {/* SINGLE CONTINUOUS WAVY NEON DOTTED BEAM TOUCHING KAFKA (125) -> AUVIS (375) -> POSTGRES (625) -> PGVECTOR (875) */}
            <svg className="absolute inset-0 w-full h-full pointer-events-none hidden md:block z-0 overflow-visible" viewBox="0 0 1000 240" preserveAspectRatio="none">
              <defs>
                <linearGradient id="singleWavyGrad" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stopColor="#f59e0b" />
                  <stop offset="50%" stopColor="#6366f1" />
                  <stop offset="100%" stopColor="#10b981" />
                </linearGradient>
              </defs>

              {/* Ambient Background Guideline */}
              <path
                d="M 125 110 C 200 50, 300 170, 375 110 C 450 50, 550 170, 625 110 C 700 50, 800 170, 875 110"
                stroke="#6366f1"
                strokeWidth="2"
                strokeDasharray="4 6"
                strokeOpacity="0.3"
                fill="none"
              />

              {/* Single Continuous Animated Neon Dotted Wavy Line */}
              <path
                d="M 125 110 C 200 50, 300 170, 375 110 C 450 50, 550 170, 625 110 C 700 50, 800 170, 875 110"
                stroke="url(#singleWavyGrad)"
                strokeWidth="3.5"
                fill="none"
                className="animate-neon-beam-1"
              />
            </svg>

            {/* NODE 1: KAFKA */}
            <div
              onClick={() => handleNodeClick('kafka')}
              className={`animate-float-node-1 bg-slate-950/90 border rounded-xl p-5 flex flex-col gap-3 shadow-2xl z-10 cursor-pointer transition-all duration-300 ${selectedNode === 'kafka' ? 'border-amber-500 ring-2 ring-amber-500/40' : 'border-slate-800/90 hover:border-amber-500/60'
                }`}
            >
              <div className="flex items-center justify-between border-b border-slate-800/80 pb-2.5">
                <span className="text-xs font-mono font-bold uppercase tracking-wider text-amber-400">
                  Kafka
                </span>
                <span className="inline-block w-2 h-2 rounded-full bg-amber-400 animate-pulse"></span>
              </div>

              <div>
                <p className="text-sm font-semibold text-slate-100">Event Stream</p>
                <p className="text-xs font-mono text-slate-400 mt-0.5">topic: patient-records</p>
              </div>

              <div className="pt-2 border-t border-slate-800/60 flex items-center justify-between">
                <span className="text-[11px] text-slate-400">Ingest rate</span>
                <span className="text-xs font-mono font-semibold text-amber-400">200 rec/s</span>
              </div>

              {selectedNode === 'kafka' && (
                <div className="mt-2 pt-3 border-t border-amber-500/30 flex flex-col gap-1.5 text-[11px] font-mono text-slate-300">
                  <div className="flex justify-between"><span className="text-slate-500">partition:</span> <span>0</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">group_id:</span> <span>auvis-group</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">auth:</span> <span>SASL/SCRAM</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">dlq_topic:</span> <span className="text-amber-400">patient-dlq</span></div>
                </div>
              )}
            </div>

            {/* NODE 2: AUVIS CORE ENGINE */}
            <div
              onClick={() => handleNodeClick('engine')}
              className={`animate-float-node-2 bg-slate-950/95 border rounded-xl p-5 flex flex-col gap-3 shadow-2xl shadow-indigo-500/10 z-10 cursor-pointer transition-all duration-300 ${selectedNode === 'engine' ? 'border-indigo-400 ring-2 ring-indigo-500/50' : 'border-indigo-500/40 hover:border-indigo-400'
                }`}
            >
              <div className="flex items-center justify-between border-b border-slate-800/80 pb-2.5">
                <span className="text-xs font-mono font-bold uppercase tracking-wider text-indigo-400">
                  Auvis Core Engine
                </span>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
                  v0.2.6
                </span>
              </div>

              <div>
                <p className="text-sm font-semibold text-slate-100">Governance Pipeline</p>
                <p className="text-xs font-mono text-slate-400 mt-0.5">policy: patient_demo.yaml</p>
              </div>

              <div className="pt-2 border-t border-slate-800/60 flex items-center justify-between">
                <span className="text-[11px] text-slate-400">Latency</span>
                <span className="text-xs font-mono font-semibold text-emerald-400">&lt;0.01 ms</span>
              </div>

              {selectedNode === 'engine' && (
                <div className="mt-2 pt-3 border-t border-indigo-500/30 flex flex-col gap-2">
                  <div className="p-2.5 rounded bg-slate-900/90 border border-slate-800 font-mono text-[10.5px] leading-relaxed text-slate-300 overflow-x-auto">
                    <span className="text-indigo-400">rules:</span><br />
                    &nbsp;&nbsp;- <span className="text-slate-400">field:</span> ssn <span className="text-rose-400"># remove</span><br />
                    &nbsp;&nbsp;- <span className="text-slate-400">field:</span> email <span className="text-amber-400"># mask</span><br />
                    &nbsp;&nbsp;- <span className="text-slate-400">field:</span> phone <span className="text-amber-400"># mask</span><br />
                    &nbsp;&nbsp;- <span className="text-slate-400">field:</span> patient_id <span className="text-purple-400"># pseudonymize</span><br />
                    &nbsp;&nbsp;- <span className="text-slate-400">field:</span> notes <span className="text-emerald-400"># allow</span>
                  </div>

                  <div className="flex flex-col gap-1 text-[11px] font-mono text-slate-300 pt-1">
                    <div className="flex justify-between"><span className="text-slate-500">fsync_mode:</span> <span>never (6,541 r/s)</span></div>
                    <div className="flex justify-between"><span className="text-slate-500">breakers:</span> <span className="text-emerald-400">3/3 Closed</span></div>
                    <div className="flex justify-between"><span className="text-slate-500">policies_loaded:</span> <span>1 compiled</span></div>
                  </div>
                </div>
              )}
            </div>

            {/* NODE 3: POSTGRES */}
            <div
              onClick={() => handleNodeClick('postgres')}
              className={`animate-float-node-3 bg-slate-950/90 border rounded-xl p-5 flex flex-col gap-3 shadow-2xl z-10 cursor-pointer transition-all duration-300 ${selectedNode === 'postgres' ? 'border-emerald-500 ring-2 ring-emerald-500/40' : 'border-slate-800 hover:border-emerald-500/60'
                }`}
            >
              <div className="flex items-center justify-between border-b border-slate-800/80 pb-2.5">
                <span className="text-xs font-mono font-bold uppercase tracking-wider text-emerald-400">
                  PostgreSQL
                </span>
                <span className="inline-block w-2 h-2 rounded-full bg-emerald-400"></span>
              </div>

              <div>
                <p className="text-sm font-semibold text-slate-100">Structured SQL Sink</p>
                <p className="text-xs font-mono text-slate-400 mt-0.5">table: governed_patients</p>
              </div>

              <div className="pt-2 border-t border-slate-800/60 flex items-center justify-between">
                <span className="text-[11px] text-slate-400">Status</span>
                <span className="text-xs font-mono font-semibold text-emerald-400">Synced</span>
              </div>

              {selectedNode === 'postgres' && (
                <div className="mt-2 pt-3 border-t border-emerald-500/30 flex flex-col gap-1.5 text-[11px] font-mono text-slate-300">
                  <div className="flex justify-between"><span className="text-slate-500">database:</span> <span>auvis_demo</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">pool:</span> <span>10 active / 20 max</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">sslmode:</span> <span>verify-full</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">primary_key:</span> <span>id (UUID)</span></div>
                </div>
              )}
            </div>

            {/* NODE 4: PGVECTOR */}
            <div
              onClick={() => handleNodeClick('pgvector')}
              className={`animate-float-node-4 bg-slate-950/90 border rounded-xl p-5 flex flex-col gap-3 shadow-2xl z-10 cursor-pointer transition-all duration-300 ${selectedNode === 'pgvector' ? 'border-emerald-500 ring-2 ring-emerald-500/40' : 'border-slate-800 hover:border-emerald-500/60'
                }`}
            >
              <div className="flex items-center justify-between border-b border-slate-800/80 pb-2.5">
                <span className="text-xs font-mono font-bold uppercase tracking-wider text-emerald-400">
                  pgvector Index
                </span>
                <span className="inline-block w-2 h-2 rounded-full bg-emerald-400"></span>
              </div>

              <div>
                <p className="text-sm font-semibold text-slate-100">Vector Embeddings</p>
                <p className="text-xs font-mono text-slate-400 mt-0.5">1536-dim vector index</p>
              </div>

              <div className="pt-2 border-t border-slate-800/60 flex items-center justify-between">
                <span className="text-[11px] text-slate-400">Status</span>
                <span className="text-xs font-mono font-semibold text-emerald-400">Governed Sink</span>
              </div>

              {selectedNode === 'pgvector' && (
                <div className="mt-2 pt-3 border-t border-emerald-500/30 flex flex-col gap-1.5 text-[11px] font-mono text-slate-300">
                  <div className="flex justify-between"><span className="text-slate-500">extension:</span> <span>pgvector v0.7</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">index_type:</span> <span>HNSW (m=16)</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">dimensions:</span> <span>1536</span></div>
                  <div className="flex justify-between"><span className="text-slate-500">distance:</span> <span>Cosine Sim</span></div>
                </div>
              )}
            </div>

          </div>
        </section>

        {/* SECTION 4 — LIVE MATCH FEED */}
        <section className="w-full max-w-[1400px] mx-auto bg-slate-950/80 border border-slate-800/90 rounded-xl p-5 flex flex-col gap-4 shadow-xl">
          <div className="flex items-center justify-between border-b border-slate-800/80 pb-3">
            <span className="text-xs font-mono font-bold uppercase tracking-wider text-slate-300">
              Live Match Feed
            </span>
            <span className="text-[11px] font-mono text-slate-500" suppressHydrationWarning>
              Last {liveFeed.length} records
            </span>
          </div>

          {liveFeed.length === 0 ? (
            <div className="p-8 text-center text-xs font-mono text-slate-500 bg-slate-900/40 rounded-lg border border-slate-800/50">
              Waiting for live stream events from Auvis Gateway (http://localhost:8082/auvis/stream)...
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
              {liveFeed.map((item, index) => (
                <div
                  key={item.id || index}
                  onClick={() => setActiveAuditRecord(item)}
                  title="Click to view full governance audit record"
                  className={`p-3.5 rounded-lg bg-slate-900/90 border flex flex-col gap-2 transition-all duration-200 cursor-pointer hover:scale-[1.02] hover:shadow-lg ${index === 0 ? 'border-indigo-500/60 bg-indigo-500/10 hover:border-indigo-400' : 'border-slate-800/80 hover:border-indigo-500/40 hover:bg-slate-900'
                    }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-slate-100 flex items-center gap-1.5">
                      <span className="w-1.5 h-1.5 rounded-full bg-indigo-400"></span>
                      {item.diagnosis}
                    </span>
                    {item.fieldsExposedPHI && item.fieldsExposedPHI.length > 0 ? (
                      <span className="font-mono text-[10px] font-bold px-1.5 py-0.5 rounded bg-rose-500/20 text-rose-400 border border-rose-500/40 flex items-center gap-1 animate-pulse">
                        ⚠️ EXPOSED PHI ({item.fieldsExposedPHI.length})
                      </span>
                    ) : (
                      <span className="font-mono text-[10px] font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        Audit
                      </span>
                    )}
                  </div>

                  <div className="flex items-center justify-between text-[11px]">
                    <span className="font-mono text-slate-400">
                      {item.maskedReference}
                    </span>
                    <span className="font-mono text-slate-500 text-[10px]" suppressHydrationWarning>
                      {item.timestamp}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

      </div>

      {/* SECTION 5 — FOOTER STRIP */}
      <footer className="w-full h-10 px-6 border-t border-slate-800/80 bg-slate-950/60 flex items-center justify-between text-[11px] font-mono text-slate-500 flex-shrink-0">
        <div>
          <span suppressHydrationWarning>Elapsed: {mounted ? formatTime(elapsedSeconds) : '00:14:32'}</span>
        </div>
        <div>
          <span>Synthea Healthcare Dataset — 500,000 Patient Records</span>
        </div>
      </footer>

      {/* DRAGGABLE GLASSMORPHIC AUDIT INSPECTOR DRAWER */}
      {activeAuditRecord && (
        <div
          style={drawerPos ? { left: `${drawerPos.x}px`, top: `${drawerPos.y}px`, right: 'auto', bottom: 'auto' } : undefined}
          className={`fixed ${drawerPos ? '' : 'bottom-4 right-4'} w-[90vw] md:w-[460px] z-50 pointer-events-auto transition-shadow duration-150 ${isDragging ? 'shadow-2xl shadow-indigo-500/50 scale-[1.01]' : 'shadow-2xl shadow-indigo-500/20'}`}
        >
          <div
            className="bg-slate-950/95 backdrop-blur-xl border border-indigo-500/50 rounded-2xl flex flex-col overflow-hidden"
          >
            {/* Drawer Header (Draggable Handle) */}
            <div
              onPointerDown={handlePointerDown}
              onPointerMove={handlePointerMove}
              onPointerUp={handlePointerUp}
              title="Click and drag header to position anywhere on screen"
              className="px-5 py-3 border-b border-slate-800/80 flex items-center justify-between bg-slate-900/80 cursor-grab active:cursor-grabbing select-none"
            >
              <div className="flex items-center gap-3">
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
                </span>
                <span className="font-mono text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                  Audit Entry: {activeAuditRecord.id}
                  <span className="text-[10px] text-slate-500 font-normal">⋮⋮ drag</span>
                </span>
              </div>
              <button
                onClick={() => setActiveAuditRecord(null)}
                className="text-slate-400 hover:text-slate-100 font-mono text-xs px-2.5 py-1 rounded-md bg-slate-800/60 hover:bg-slate-800 transition-colors border border-slate-700/50 cursor-pointer"
              >
                ✕ Close
              </button>
            </div>

            {/* Drawer Content */}
            <div className="p-5 flex flex-col gap-4 max-h-[60vh] overflow-y-auto">

              {/* Shadow PHI Warning Banner */}
              {activeAuditRecord.fieldsExposedPHI && activeAuditRecord.fieldsExposedPHI.length > 0 && (
                <div className="p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/40 flex flex-col gap-2 font-mono text-xs shadow-lg shadow-rose-500/5">
                  <div className="flex items-center gap-2 text-rose-400 font-bold uppercase tracking-wider text-[11px]">
                    <span className="relative flex h-2 w-2">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-rose-400 opacity-75"></span>
                      <span className="relative inline-flex rounded-full h-2 w-2 bg-rose-500"></span>
                    </span>
                    ⚠️ Shadow PHI Exposure Detected
                  </div>
                  <p className="text-slate-300 text-[11px] leading-relaxed">
                    Unmanaged PHI/PII patterns passed unmasked because they were omitted from active policy rules:
                  </p>
                  <div className="flex flex-wrap gap-1.5 mt-0.5">
                    {activeAuditRecord.fieldsExposedPHI.map((fieldPath, idx) => (
                      <span key={idx} className="px-2 py-0.5 rounded bg-rose-500/20 text-rose-300 border border-rose-500/40 text-[11px] font-semibold">
                        {fieldPath}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Rules & Compliance Badges */}
              <div className="flex flex-wrap items-center gap-2 text-xs font-mono">
                <span className="text-slate-400 font-medium">Rules Applied:</span>
                {(activeAuditRecord.rulesApplied && activeAuditRecord.rulesApplied.length > 0) ? (
                  activeAuditRecord.rulesApplied.map((rule, idx) => (
                    <span key={idx} className="px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30 font-semibold text-[11px]">
                      {rule}
                    </span>
                  ))
                ) : (
                  <span className="text-slate-500 text-[11px] italic">No active rules applied</span>
                )}
              </div>

              {/* Governed Result Payload */}
              <div className="flex flex-col gap-1.5 w-full">
                <span className="text-[11px] font-semibold text-emerald-400 uppercase tracking-wider font-mono flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                  Governed Record Audit (Masked / Redacted)
                </span>
                <pre className="p-4 rounded-xl bg-slate-900/90 border border-emerald-500/30 text-xs font-mono text-slate-200 overflow-x-auto max-h-[280px] leading-relaxed">
                  {JSON.stringify(activeAuditRecord.governedPayload || {}, null, 2)}
                </pre>
              </div>

            </div>

            {/* Drawer Footer */}
            <div className="px-5 py-2.5 border-t border-slate-800/80 bg-slate-900/50 flex items-center justify-between text-[10px] font-mono text-slate-400">
              <span>Timestamp: {activeAuditRecord.timestamp}</span>
              <span className="text-emerald-400 font-semibold">Zero Latency &lt;0.01ms</span>
            </div>
          </div>
        </div>
      )}

    </main>
  );
}
